package main

import (
	"fmt"
	"html/template"
	"net/http"
	"sync"
)

var (
	messages   []Message
	users      []string
	usersMutex sync.Mutex
	clients    []chan Message
)

type Message struct {
	User    string
	Content string
}

func main() {
	// Обработчик для статических файлов (например, CSS)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	http.HandleFunc("/", handleRoot)
	http.HandleFunc("/submit", handleSubmit)
	http.HandleFunc("/events", handleEvents)
	http.HandleFunc("/login", handleLogin)

	fmt.Println("Server started at :28080")
	http.ListenAndServe(":28080", nil)
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	// Проверяем, есть ли имя пользователя в сессии
	username := r.URL.Query().Get("username")

	if username == "" {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Загружаем HTML-шаблон
	tmpl := template.Must(template.New("index").Parse(`
		<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<title>Welcome</title>
			<link rel="stylesheet" href="/static/styles.css">
			<script>
				// Подключаемся к SSE
				const eventSource = new EventSource("/events");
				eventSource.onmessage = function(event) {
					// Обновляем список сообщений
					const messageList = document.getElementById("messageList");
					const newMessage = document.createElement("li");
					const messageData = JSON.parse(event.data);
					newMessage.textContent = messageData.user + ": " + messageData.content;
					messageList.appendChild(newMessage);
				};
			</script>
		</head>
		<body>
			<div class="container">
				<h1>Welcome, {{.Username}}!</h1>
				<form action="/submit" method="POST">
					<input type="hidden" name="username" value="{{.Username}}">
					<input type="text" name="content" required placeholder="Type your message...">
					<button type="submit">Send</button>
				</form>
				
				<h2>Messages:</h2>
				<ul id="messageList">
					{{range .Messages}}
						<li>{{.User}}: {{.Content}}</li>
					{{end}}
				</ul>
			</div>
		</body>
		</html>
	`))

	// Отправляем имя пользователя и сообщения в шаблон
	tmpl.Execute(w, struct {
		Username string
		Messages []Message
	}{
		Username: username,
		Messages: messages,
	})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		username := r.FormValue("username")
		http.Redirect(w, r, "/?username="+username, http.StatusSeeOther)
		return
	}

	// Отправляем форму для ввода имени
	tmpl := template.Must(template.New("login").Parse(`
		<!DOCTYPE html>
		<html lang="en">
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<title>Login</title>
		</head>
		<body>
			<form method="POST">
				<label for="username">Enter your name:</label>
				<input type="text" name="username" required>
				<button type="submit">Login</button>
			</form>
		</body>
		</html>
	`))

	tmpl.Execute(w, nil)
}

func handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// Получаем имя и сообщение из формы
		username := r.FormValue("username")
		content := r.FormValue("content")

		// Защищаем от конкурентного доступа
		usersMutex.Lock()
		messages = append(messages, Message{User: username, Content: content}) // Сохраняем сообщение
		usersMutex.Unlock()

		// Отправляем обновление всем клиентам через SSE
		for _, client := range clients {
			client <- Message{User: username, Content: content}
		}
	}

	// Перенаправляем обратно на главную страницу
	http.Redirect(w, r, "/?username="+r.FormValue("username"), http.StatusSeeOther)
}

func handleEvents(w http.ResponseWriter, r *http.Request) {
	// Устанавливаем заголовки для SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Канал для отправки сообщений клиентам
	clientChan := make(chan Message)
	clients = append(clients, clientChan)

	// Когда клиент закроет соединение, удаляем его из списка клиентов
	defer func() {
		for i, c := range clients {
			if c == clientChan {
				clients = append(clients[:i], clients[i+1:]...)
				break
			}
		}
	}()

	// Отправляем новые сообщения
	for {
		msg := <-clientChan
		// Отправляем сообщение всем подключенным клиентам
		fmt.Fprintf(w, "data: %s\n\n", fmt.Sprintf(`{"user":"%s", "content":"%s"}`, msg.User, msg.Content))
		// Принудительно заставляем сервер отправить данные клиенту
		flusher, ok := w.(http.Flusher)
		if ok {
			flusher.Flush()
		}
	}
}

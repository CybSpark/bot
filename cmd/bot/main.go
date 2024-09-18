package main

import "fmt"

func main() {
	printer("hello world")
}

func printer(v interface{}) {
	fmt.Println(v)
}

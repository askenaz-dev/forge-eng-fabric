package main

import (
    "flag"
    "fmt"
)

func main() {
    name := flag.String("name", "example", "template name")
    flag.Parse()
    fmt.Printf("scaffolder stub: render template %s\n", *name)
}

package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage:")
		fmt.Println("keygen <filePath>")
		return
	}
	gen()
}

func gen() {
	filePath := os.Args[1]

	keySize := 32 // 256-bit AES key
	key := make([]byte, keySize)

	// Use the crypto/rand package to generate a random key
	_, err := io.ReadFull(rand.Reader, key)
	if err != nil {
		fmt.Println("Error generating random key:", err)
		return
	}

	err = os.WriteFile(filePath, key, 0644)
	if err != nil {
		fmt.Println("Error writing file:" + err.Error())
	}
}

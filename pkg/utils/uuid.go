package utils

import gonanoid "github.com/matoous/go-nanoid/v2"

const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func GenerateID() (string, error) {
	return gonanoid.Generate(characters, 6)
}

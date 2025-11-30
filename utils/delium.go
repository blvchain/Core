package utils

import (
	"crypto/sha256"
	"crypto/sha512"
	"errors"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type D_hash struct {
	Byte_slice       []byte
	String           string
	Primitive_binary primitive.Binary
}

func D256C(data primitive.Binary, path string) (D_hash, error) {

	current := sha256.Sum256(data.Data)
	hashBytes := current[:]

	if path != "" {
		steps := strings.Split(path, "/")

		for _, step := range steps {
			if step == "" {
				continue
			}

			parts := strings.Split(step, "#")
			if len(parts) != 2 {
				return D_hash{}, errors.New("invalid path segment")
			}

			addon := []byte(parts[0])

			deleteStep, err := strconv.Atoi(parts[1])
			if err != nil {
				return D_hash{}, errors.New("invalid delete step")
			}

			merged := make([]byte, 0, len(hashBytes)+len(addon))
			merged = append(merged, hashBytes...)
			merged = append(merged, addon...)

			h := sha256.Sum256(merged)
			hashBytes = h[:]

			if deleteStep > 0 {
				if deleteStep >= len(hashBytes) {
					return D_hash{}, errors.New("delete step too large")
				}
				hashBytes = hashBytes[:len(hashBytes)-deleteStep]
			}
		}
	}

	hexString := ByteToHexString(hashBytes)

	return D_hash{
		Byte_slice:       hashBytes,
		String:           hexString,
		Primitive_binary: ToMongoBinary(hashBytes),
	}, nil
}

func D512C(data primitive.Binary, path string) (D_hash, error) {

	current := sha512.Sum512(data.Data)
	hashBytes := current[:]

	if path != "" {
		steps := strings.Split(path, "/")

		for _, step := range steps {
			if step == "" {
				continue
			}

			parts := strings.Split(step, "#")
			if len(parts) != 2 {
				return D_hash{}, errors.New("invalid path segment")
			}

			addon := []byte(parts[0])

			deleteStep, err := strconv.Atoi(parts[1])
			if err != nil {
				return D_hash{}, errors.New("invalid delete step")
			}

			merged := make([]byte, 0, len(hashBytes)+len(addon))
			merged = append(merged, hashBytes...)
			merged = append(merged, addon...)

			h := sha512.Sum512(merged)
			hashBytes = h[:]

			if deleteStep > 0 {
				if deleteStep >= len(hashBytes) {
					return D_hash{}, errors.New("delete step too large")
				}
				hashBytes = hashBytes[:len(hashBytes)-deleteStep]
			}
		}
	}

	hexString := ByteToHexString(hashBytes)

	return D_hash{
		Byte_slice:       hashBytes,
		String:           hexString,
		Primitive_binary: ToMongoBinary(hashBytes),
	}, nil
}

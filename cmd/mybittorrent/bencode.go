package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/golang-collections/collections/stack"
	"github.com/niemeyer/golang/src/pkg/container/vector"
)

func decodeString(bencodedString string) (string, int, error) {
	var firstColonIndex int

	for i := 0; i < len(bencodedString); i++ {
		if bencodedString[i] == ':' {
			firstColonIndex = i
			break
		}
	}

	lengthStr := bencodedString[:firstColonIndex]

	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return "", 0, fmt.Errorf("Failed to convert string length from [%s] (%w)", lengthStr, err)
	}
	resultStr := bencodedString[firstColonIndex+1 : firstColonIndex+1+length]
	totalLength := len(lengthStr) + len(resultStr) + 1
	return resultStr, totalLength, nil
}

func decodeInt(bencodedString string) (int, int, error) {
	finalPos := strings.Index(bencodedString, "e")
	numberAsStr := bencodedString[1:finalPos]
	resultInt, err := strconv.Atoi(numberAsStr)
	if err != nil {
		return 0, 0, fmt.Errorf("Failed to convert integer from [%s] (%w)", numberAsStr, err)
	}

	return resultInt, finalPos + 1, nil
}

func addToStack(st *stack.Stack, value any) {
	top := st.Peek()
	switch top.(type) {
	case *vector.Vector:
		top.(*vector.Vector).Push(value)
	case map[string]any:
		st.Push(value.(string))
	case string:
		key := st.Pop().(string)
		nextTop := st.Peek().(map[string]any)
		nextTop[key] = value
	}
}

func Decode(reader io.Reader) (string, error) {
	var originalVector vector.Vector
	st := stack.New()
	st.Push(&originalVector)

	bencodedStringBinary, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("Faield to read file (%w)", err)
	}
	bencodedString := string(bencodedStringBinary)

	pos := 0
	for {
		if pos >= len(bencodedString) {
			break
		}
		if unicode.IsDigit(rune(bencodedString[pos])) {
			value, skip, err := decodeString(bencodedString[pos:])
			if err != nil {
				return "", fmt.Errorf("Failed to decode string at pos %d (%w)", pos, err)
			}
			addToStack(st, value)
			pos += skip
			continue
		} else if bencodedString[pos] == 'i' {
			value, skip, err := decodeInt(bencodedString[pos:])
			if err != nil {
				return "", fmt.Errorf("Failed to decode integer at pos %d (%w)", pos, err)
			}
			addToStack(st, value)
			pos += skip
			continue
		} else if bencodedString[pos] == 'l' {
			var newVector vector.Vector
			st.Push(&newVector)
			pos += 1
			continue
		} else if bencodedString[pos] == 'd' {
			newMap := make(map[string]any)
			st.Push(newMap)
			pos += 1
			continue
		} else if bencodedString[pos] == 'e' {
			finishedObject := st.Pop()
			switch finishedObject.(type) {
			case *vector.Vector:
				finishedVector := *finishedObject.(*vector.Vector)
				if len(finishedVector) == 0 {
					addToStack(st, []int8{})
				} else {
					addToStack(st, finishedVector)
				}
			case map[string]any:
				finishedMap := finishedObject.(map[string]any)
				addToStack(st, finishedMap)
			default:
				return "", errors.New(fmt.Sprintf("Unknown type to finish at pos %d", pos))
			}
			pos += 1
			continue
		} else {
			return "", fmt.Errorf("Unsupported")
		}
	}

	var value interface{}
	if len(originalVector) == 1 {
		value = originalVector[0]
	} else {
		value = originalVector
	}
	decoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("Failed to decode final value (%w)", err)
	}
	return string(decoded), nil
}

func ToSnakeCase(str string) string {
	var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

func Unmarshal(reader io.Reader, v any) error {
	data, err := Decode(reader)
	if err != nil {
		return fmt.Errorf("Failed to decode to str (%w)", err)
	}
	return json.Unmarshal([]byte(data), v)
}

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/golang-collections/collections/stack"
	"github.com/niemeyer/golang/src/pkg/container/vector"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

type AllMap map[string]interface{}

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

func addToStack(st *stack.Stack, value interface{}) {
	top := st.Peek()
	switch top.(type) {
	case *vector.Vector:
		top.(*vector.Vector).Push(value)
	case AllMap:
		st.Push(value.(string))
	case string:
		key := st.Pop().(string)
		nextTop := st.Peek().(AllMap)
		nextTop[key] = value
	}
}

func decodeBencode(bencodedString string) (interface{}, error) {
	var originalVector vector.Vector
	st := stack.New()
	st.Push(&originalVector)

	pos := 0
	for {
		if pos >= len(bencodedString) {
			break
		}
		if unicode.IsDigit(rune(bencodedString[pos])) {
			value, skip, err := decodeString(bencodedString[pos:])
			if err != nil {
				return nil, fmt.Errorf("Failed to decode string at pos %d (%w)", pos, err)
			}
			addToStack(st, value)
			pos += skip
			continue
		} else if bencodedString[pos] == 'i' {
			value, skip, err := decodeInt(bencodedString[pos:])
			if err != nil {
				return nil, fmt.Errorf("Failed to decode integer at pos %d (%w)", pos, err)
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
			newMap := make(AllMap)
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
			case AllMap:
				finishedMap := finishedObject.(AllMap)
				addToStack(st, finishedMap)
			default:
				return nil, errors.New(fmt.Sprintf("Unknown type to finish at pos %d", pos))
			}
			pos += 1
			continue
		} else {
			return nil, fmt.Errorf("Unsupported")
		}
	}
	if len(originalVector) == 1 {
		return originalVector[0], nil
	} else {
		return originalVector, nil
	}
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		value, err := decodeBencode(bencodedValue)
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(value)
		fmt.Println(string(jsonOutput))
	} else if command == "info" {
		filePath := os.Args[2]
		f, err := os.Open(filePath)
		if err != nil {
			fmt.Println(err)
			return
		}
		bencodedValue, err := io.ReadAll(f)
		if err != nil {
			fmt.Println(err)
			return
		}

		value, err := decodeBencode(string(bencodedValue))
		if err != nil {
			fmt.Println(err)
			return
		}

		asMap := value.(AllMap)
		fmt.Printf("Tracker URL: %s\n", asMap["announce"])
		fmt.Printf("Length: %d\n", asMap["info"].(AllMap)["length"])

	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}

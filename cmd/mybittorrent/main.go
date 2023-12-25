package main

import (
	"encoding/json"
	"errors"
	"fmt"
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
		return "", 0, err
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
		return 0, 0, fmt.Errorf("Failed to convert integer: %s", numberAsStr, err)
	}

	return resultInt, finalPos + 1, nil
}

func decodeList(bencodedString string) ([]interface{}, error) {
	return nil, nil
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

func decodeBencode(bencodedString string) (vector.Vector, error) {
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
				if len(finishedMap) == 0 {
					addToStack(st, make(map[int8]int8))
				} else {
					addToStack(st, finishedMap)
				}
			default:
				return nil, errors.New(fmt.Sprintf("Unknown type to finish at pos %d", pos))
			}
			pos += 1
			continue
		} else {
			return nil, fmt.Errorf("Unsupported")
		}
	}
	return originalVector, nil
}

func main() {
	command := os.Args[1]

	if command == "decode" {
		bencodedValue := os.Args[2]

		finalArray, err := decodeBencode(bencodedValue)
		if err != nil {
			fmt.Println(err)
			return
		}
		var decoded interface{}
		if len(finalArray) == 1 {
			decoded = finalArray[0]
		} else {
			decoded = finalArray
		}
		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}

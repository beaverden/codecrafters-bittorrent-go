package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/golang-collections/collections/stack"
	"github.com/niemeyer/golang/src/pkg/container/vector"
)

type BencodeDecoder struct {
	reader io.Reader
}

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
	case map[string]interface{}:
		st.Push(value.(string))
	case string:
		key := st.Pop().(string)
		nextTop := st.Peek().(map[string]interface{})
		nextTop[key] = value
	}
}

func NewDecoder(reader io.Reader) *BencodeDecoder {
	return &BencodeDecoder{reader: reader}
}

func (bd *BencodeDecoder) DecodeToStr(v *string) error {
	var originalVector vector.Vector
	st := stack.New()
	st.Push(&originalVector)

	bencodedStringBinary, err := io.ReadAll(bd.reader)
	if err != nil {
		return fmt.Errorf("Faield to read file (%w)", err)
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
				return fmt.Errorf("Failed to decode string at pos %d (%w)", pos, err)
			}
			addToStack(st, value)
			pos += skip
			continue
		} else if bencodedString[pos] == 'i' {
			value, skip, err := decodeInt(bencodedString[pos:])
			if err != nil {
				return fmt.Errorf("Failed to decode integer at pos %d (%w)", pos, err)
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
			newMap := make(map[string]interface{})
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
			case map[string]interface{}:
				finishedMap := finishedObject.(map[string]interface{})
				addToStack(st, finishedMap)
			default:
				return errors.New(fmt.Sprintf("Unknown type to finish at pos %d", pos))
			}
			pos += 1
			continue
		} else {
			return fmt.Errorf("Unsupported")
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
		return fmt.Errorf("Failed to decode final value (%w)", err)
	}
	*v = string(decoded)

	return nil
}

func (bd *BencodeDecoder) DecodeToStruct(v any) error {
	var s string
	err := bd.DecodeToStr(&s)
	if err != nil {
		return fmt.Errorf("Failed to decode to str (%w)", err)
	}
	return json.Unmarshal([]byte(s), v);
}

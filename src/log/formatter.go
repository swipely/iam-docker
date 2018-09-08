package log

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"strconv"
	"time"
)

// Formatter is used by logrus to turn entries into logs.
type Formatter struct{}

// Format displays a logrus.Entry.
func (f *Formatter) Format(entry *logrus.Entry) ([]byte, error) {
	buffer := new(bytes.Buffer)
	prefix, ok := entry.Data["prefix"].(string)
	if !ok {
		return []byte{}, errors.New("Invalid log prefix")
	}
	_, err := buffer.WriteString(fmt.Sprint(entry.Time.Format(time.RFC3339), " [", escapeIfNeeded(prefix), "] ", entry.Message))

	if err != nil {
		return []byte{}, err
	}

	for k, v := range entry.Data {
		key := escapeIfNeeded(k)
		val := escapeIfNeeded(fmt.Sprint(v))
		if key != "prefix" {
			_, err = buffer.WriteString(fmt.Sprint(" ", key, "=", val))
			if err != nil {
				return []byte{}, err
			}
		}
	}

	_, err = buffer.WriteString("\n")
	if err != nil {
		return []byte{}, err
	}

	return buffer.Bytes(), nil
}

func escapeIfNeeded(str string) string {
	needed := false

	for _, char := range str {
		if escapeNeeded(char) {
			needed = true
			break
		}
	}

	if !needed {
		return str
	}

	return strconv.Quote(str)
}

func escapeNeeded(char rune) bool {
	return !(((char >= '!') && (char <= '^')) || ((char >= '_') && (char <= '~')))
}

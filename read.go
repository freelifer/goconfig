package goconfig

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// readError occurs when read configuration file with wrong format.
type readError struct {
	Reason  ParseError
	Content string // Line content
}

// Error implement Error interface.
func (err readError) Error() string {
	switch err.Reason {
	case ERR_BLANK_SECTION_NAME:
		return "empty section name not allowed"
	case ERR_COULD_NOT_PARSE:
		return fmt.Sprintf("could not parse line: %s", string(err.Content))
	}
	return "invalid read error"
}

// LoadConfigFile reads a file and returns a new configuration representation.
// This representation can be queried with GetValue.
func LoadConfigFile(fileName string, moreFiles ...string) (c *ConfigFile, err error) {
	// Append files' name together.
	fileNames := make([]string, 1, len(moreFiles)+1)
	fileNames[0] = fileName
	if len(moreFiles) > 0 {
		fileNames = append(fileNames, moreFiles...)
	}

	c = newConfigFile(fileNames)

	for _, name := range fileNames {
		if err = c.loadFile(name); err != nil {
			return nil, err
		}
	}

	return c, nil
}

func (c *ConfigFile) loadFile(fileName string) (err error) {
	f, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer f.Close()

	return c.read(f)
}

// Read reads an io.Reader and returns a configuration representation.
// This representation can be queried with GetValue.
func (c *ConfigFile) read(reader io.Reader) (err error) {
	buf := bufio.NewReader(reader)

	// Handle BOM-UTF8.
	// http://en.wikipedia.org/wiki/Byte_order_mark#Representations_of_byte_order_marks_by_encoding
	mask, err := buf.Peek(3)
	if err == nil && len(mask) >= 3 &&
		mask[0] == 239 && mask[1] == 187 && mask[2] == 191 {
		buf.Read(mask)
	}

	count := 1 // Counter for auto increment.
	// Current section name.
	section := DEFAULT_SECTION
	var comments string
	// Parse line-by-line
	for {
		line, err := buf.ReadString('\n')
		line = strings.TrimSpace(line)
		lineLengh := len(line) //[SWH|+]
		if err != nil {
			if err != io.EOF {
				return err
			}

			// Reached end of file, if nothing to read then break,
			// otherwise handle the last line.
			if lineLengh == 0 {
				break
			}
		}

		// switch written for readability (not performance)
		switch {
		case lineLengh == 0: // Empty line
			continue
		case line[0] == '#' || line[0] == ';': // Comment
			// Append comments
			if len(comments) == 0 {
				comments = line
			} else {
				comments += LineBreak + line
			}
			continue
		case line[0] == '[' && line[lineLengh-1] == ']': // New sction.
			// Get section name.
			section = strings.TrimSpace(line[1 : lineLengh-1])
			// Set section comments and empty if it has comments.
			if len(comments) > 0 {
				c.setSectionComments(section, comments)
				comments = ""
			}
			// Make section exist even though it does not have any key.
			c.setValue(section, " ", " ")
			// Reset counter.
			count = 1
			continue
		case section == "": // No section defined so far
			return readError{ERR_BLANK_SECTION_NAME, line}
		default: // Other alternatives
			var (
				i        int
				keyQuote string
				key      string
				valQuote string
				value    string
			)
			//[SWH|+]:支持引号包围起来的字串
			if line[0] == '"' {
				if lineLengh >= 6 && line[0:3] == `"""` {
					keyQuote = `"""`
				} else {
					keyQuote = `"`
				}
			} else if line[0] == '`' {
				keyQuote = "`"
			}
			if keyQuote != "" {
				qLen := len(keyQuote)
				pos := strings.Index(line[qLen:], keyQuote)
				if pos == -1 {
					return readError{ERR_COULD_NOT_PARSE, line}
				}
				pos = pos + qLen
				i = strings.IndexAny(line[pos:], "=:")
				if i <= 0 {
					return readError{ERR_COULD_NOT_PARSE, line}
				}
				i = i + pos
				key = line[qLen:pos] //保留引号内的两端的空格
			} else {
				i = strings.IndexAny(line, "=:")
				if i <= 0 {
					return readError{ERR_COULD_NOT_PARSE, line}
				}
				key = strings.TrimSpace(line[0:i])
			}
			//[SWH|+];

			// Check if it needs auto increment.
			if key == "-" {
				key = "#" + fmt.Sprint(count)
				count++
			}

			//[SWH|+]:支持引号包围起来的字串
			lineRight := strings.TrimSpace(line[i+1:])
			lineRightLength := len(lineRight)
			firstChar := ""
			if lineRightLength >= 2 {
				firstChar = lineRight[0:1]
			}
			if firstChar == "`" {
				valQuote = "`"
			} else if lineRightLength >= 6 && lineRight[0:3] == `"""` {
				valQuote = `"""`
			}
			if valQuote != "" {
				qLen := len(valQuote)
				pos := strings.LastIndex(lineRight[qLen:], valQuote)
				if pos == -1 {
					return readError{ERR_COULD_NOT_PARSE, line}
				}
				pos = pos + qLen
				value = lineRight[qLen:pos]
			} else {
				value = strings.TrimSpace(lineRight[0:])
			}
			//[SWH|+];

			c.setValue(section, key, value)
			// Set key comments and empty if it has comments.
			if len(comments) > 0 {
				c.setKeyComments(section, key, comments)
				comments = ""
			}
		}

		// Reached end of file.
		if err == io.EOF {
			break
		}
	}
	return nil
}

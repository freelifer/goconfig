package goconfig

import (
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

const (
	// Default section name.
	DEFAULT_SECTION = "DEFAULT"
	// Maximum allowed depth when recursively substituing variable names.
	_DEPTH_VALUES = 200
)

type ParseError int

const (
	ERR_SECTION_NOT_FOUND ParseError = iota + 1
	ERR_KEY_NOT_FOUND
	ERR_BLANK_SECTION_NAME
	ERR_COULD_NOT_PARSE
)

var LineBreak = "\n"
var cf *ConfigFile

// Variable regexp pattern: %(variable)s
var varPattern = regexp.MustCompile(`%\(([^\)]+)\)s`)

// getError occurs when get value in configuration file with invalid parameter.
type getError struct {
	Reason ParseError
	Name   string
}

// Error implements Error interface.
func (err getError) Error() string {
	switch err.Reason {
	case ERR_SECTION_NOT_FOUND:
		return fmt.Sprintf("section '%s' not found", err.Name)
	case ERR_KEY_NOT_FOUND:
		return fmt.Sprintf("key '%s' not found", err.Name)
	}
	return "invalid get error"
}

func init() {
	if runtime.GOOS == "windows" {
		LineBreak = "\r\n"
	}
	var err error

	cf, err = LoadConfigFile("conf/app.conf")
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(cf)
}

// A ConfigFile represents a INI formar configuration file.
type ConfigFile struct {
	lock      sync.RWMutex                 // Go map is not safe.
	fileNames []string                     // Support mutil-files.
	data      map[string]map[string]string // Section -> key : value

	// Lists can keep sections and keys in order.
	sectionList []string            // Section name list.
	keyList     map[string][]string // Section -> Key name list

	sectionComments map[string]string            // Sections comments.
	keyComments     map[string]map[string]string // Keys comments.
	BlockMode       bool                         // Indicates whether use lock or not.
}

// Value return string type value.
func Value(section, key string) (string, error) {
	value, err := cf.getValue(section, key)
	return value, err
}

// Bool returns bool type value.
func Bool(section, key string) (bool, error) {
	value, err := cf.getValue(section, key)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(value)
}

// Float64 returns float64 type value.
func Float64(section, key string) (float64, error) {
	value, err := cf.getValue(section, key)
	if err != nil {
		return 0.0, err
	}
	return strconv.ParseFloat(value, 64)
}

// Int returns int type value.
func Int(section, key string) (int, error) {
	value, err := cf.getValue(section, key)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(value)
}

// Int64 returns int64 type value.
func Int64(section, key string) (int64, error) {
	value, err := cf.getValue(section, key)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(value, 10, 64)
}

// MustValue always returns value without error.
// It returns empty string if error occurs, or the default value if given.
func MustValue(section, key string, defaultVal ...string) string {
	val, err := cf.getValue(section, key)
	if len(defaultVal) > 0 && (err != nil || len(val) == 0) {
		return defaultVal[0]
	}
	return val
}

// MustBool always returns value without error,
// it returns false if error occurs.
func MustBool(section, key string, defaultVal ...bool) bool {
	val, err := Bool(section, key)
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return val
}

// MustFloat64 always returns value without error,
// it returns 0.0 if error occurs.
func MustFloat64(section, key string, defaultVal ...float64) float64 {
	value, err := Float64(section, key)
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return value
}

// MustInt always returns value without error,
// it returns 0 if error occurs.
func MustInt(section, key string, defaultVal ...int) int {
	value, err := Int(section, key)
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return value
}

// MustInt64 always returns value without error,
// it returns 0 if error occurs.
func MustInt64(section, key string, defaultVal ...int64) int64 {
	value, err := Int64(section, key)
	if len(defaultVal) > 0 && err != nil {
		return defaultVal[0]
	}
	return value
}

// newConfigFile creates an empty configuration representation.
func newConfigFile(fileNames []string) *ConfigFile {
	c := new(ConfigFile)
	c.fileNames = fileNames
	c.data = make(map[string]map[string]string)
	c.keyList = make(map[string][]string)
	c.sectionComments = make(map[string]string)
	c.keyComments = make(map[string]map[string]string)
	c.BlockMode = true
	return c
}

// SetSectionComments adds new section comments to the configuration.
// If comments are empty(0 length), it will remove its section comments!
// It returns true if the comments were inserted or removed,
// or returns false if the comments were overwritten.
func (c *ConfigFile) setSectionComments(section, comments string) bool {
	// Blank section name represents DEFAULT section.
	if len(section) == 0 {
		section = DEFAULT_SECTION
	}

	if len(comments) == 0 {
		if _, ok := c.sectionComments[section]; ok {
			delete(c.sectionComments, section)
		}

		// Not exists can be seen as remove.
		return true
	}

	// Check if comments exists.
	_, ok := c.sectionComments[section]
	if comments[0] != '#' && comments[0] != ';' {
		comments = "; " + comments
	}
	c.sectionComments[section] = comments
	return !ok
}

// getValue returns the value of key available in the given section.
// If the value needs to be unfolded
// (see e.g. %(google)s example in the GoConfig_test.go),
// then String does this unfolding automatically, up to
// _DEPTH_VALUES number of iterations.
// It returns an error and empty string value if the section does not exist,
// or key does not exist in DEFAULT and current sections.
func (c *ConfigFile) getValue(section, key string) (string, error) {
	if c.BlockMode {
		c.lock.RLock()
		defer c.lock.RUnlock()
	}

	// Blank section name represents DEFAULT section.
	if len(section) == 0 {
		section = DEFAULT_SECTION
	}

	// Check if section exists
	if _, ok := c.data[section]; !ok {
		// Section does not exist.
		return "", getError{ERR_SECTION_NOT_FOUND, section}
	}

	// Section exists.
	// Check if key exists or empty value.
	value, ok := c.data[section][key]
	if !ok {
		// Check if it is a sub-section.
		if i := strings.LastIndex(section, "."); i > -1 {
			return c.getValue(section[:i], key)
		}

		// Return empty value.
		return "", getError{ERR_KEY_NOT_FOUND, key}
	}

	// Key exists.
	var i int
	for i = 0; i < _DEPTH_VALUES; i++ {
		vr := varPattern.FindString(value)
		if len(vr) == 0 {
			break
		}

		// Take off leading '%(' and trailing ')s'.
		noption := strings.TrimLeft(vr, "%(")
		noption = strings.TrimRight(noption, ")s")

		// Search variable in default section.
		nvalue, err := c.getValue(DEFAULT_SECTION, noption)
		if err != nil && section != DEFAULT_SECTION {
			// Search in the same section.
			if _, ok := c.data[section][noption]; ok {
				nvalue = c.data[section][noption]
			}
		}

		// Substitute by new value and take off leading '%(' and trailing ')s'.
		value = strings.Replace(value, vr, nvalue, -1)
	}
	return value, nil
}

// SetValue adds a new section-key-value to the configuration.
// It returns true if the key and value were inserted,
// or returns false if the value was overwritten.
// If the section does not exist in advance, it will be created.
func (c *ConfigFile) setValue(section, key, value string) bool {
	// Blank section name represents DEFAULT section.
	if len(section) == 0 {
		section = DEFAULT_SECTION
	}
	if len(key) == 0 {
		return false
	}

	if c.BlockMode {
		c.lock.Lock()
		defer c.lock.Unlock()
	}

	// Check if section exists.
	if _, ok := c.data[section]; !ok {
		// Execute add operation.
		c.data[section] = make(map[string]string)
		// Append section to list.
		c.sectionList = append(c.sectionList, section)
	}

	// Check if key exists.
	_, ok := c.data[section][key]
	c.data[section][key] = value
	if !ok {
		// If not exists, append to key list.
		c.keyList[section] = append(c.keyList[section], key)
	}
	return !ok
}

// SetKeyComments adds new section-key comments to the configuration.
// If comments are empty(0 length), it will remove its section-key comments!
// It returns true if the comments were inserted or removed,
// or returns false if the comments were overwritten.
// If the section does not exist in advance, it is created.
func (c *ConfigFile) setKeyComments(section, key, comments string) bool {
	// Blank section name represents DEFAULT section.
	if len(section) == 0 {
		section = DEFAULT_SECTION
	}

	// Check if section exists.
	if _, ok := c.keyComments[section]; ok {
		if len(comments) == 0 {
			if _, ok := c.keyComments[section][key]; ok {
				delete(c.keyComments[section], key)
			}

			// Not exists can be seen as remove.
			return true
		}
	} else {
		if len(comments) == 0 {
			// Not exists can be seen as remove.
			return true
		} else {
			// Execute add operation.
			c.keyComments[section] = make(map[string]string)
		}
	}

	// Check if key exists.
	_, ok := c.keyComments[section][key]
	if comments[0] != '#' && comments[0] != ';' {
		comments = "; " + comments
	}
	c.keyComments[section][key] = comments
	return !ok
}

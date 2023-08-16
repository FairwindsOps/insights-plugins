package util

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/afero"
)

// UniqueFilename returns a timestamp filename in unix nanoseconds,
// and an optional suffix in the event of collisions
func UniqueFilename(fsWrapper afero.Fs, f string) string {
	// handle empty strings by returning, otherwise this will
	// not terminate
	if f == "" {
		return f
	}

	uniqueFilename := f
	_, err := fsWrapper.Stat(uniqueFilename)
	// while the file exists, we need to find a unique name
	for i := 0; err == nil; i++ {
		uniqueFilename = incrementString(uniqueFilename, i)
		_, err = fsWrapper.Stat(uniqueFilename)
	}

	return uniqueFilename
}

// incrementString returns a unique string given a string, followed
// by possibly a separator then an integer
func incrementString(str string, first int) string {
	if str == "" {
		return str
	}

	fullFilename := strings.SplitN(str, ".", 2)
	var extension string
	if len(fullFilename) > 1 {
		extension = fmt.Sprintf(".%s", fullFilename[1])
	} else {
		extension = ""
	}

	nameWithoutExtension := fullFilename[0]

	separator := "_"

	// suffix will always start at 1
	if first == 0 || first < 0 {
		first = 1
	}

	strSep := strings.SplitN(nameWithoutExtension, separator, 2)
	if len(strSep) > 1 {
		i, err := strconv.Atoi(strSep[1])

		if err != nil {
			return ""
		}

		inc := i + first
		return fmt.Sprintf("%s%s%d%s", strSep[0], separator, inc, extension)
	} else {
		return fmt.Sprintf("%s%s%d%s", nameWithoutExtension, separator, first, extension)
	}
}

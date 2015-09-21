package logger

import (
	"fmt"
	"log"
	"regexp"
	"runtime"
	"strings"
)

func Log(message string) {
	_, caller, line, ok := runtime.Caller(1)
	if ok {
		tr := regexp.MustCompile(`(/([^/]*))+$`)
		matches := tr.FindStringSubmatch(caller)

		log.Println(fmt.Sprint(matches[2], ":", line), message)
	} else {
		log.Println(message)
	}
}

func LogFn(funcname string, args ...string) {
	for i, anArg := range args {
		args[i] = fmt.Sprint("\"", anArg, "\"")
	}

	message := fmt.Sprint(funcname, "(", strings.Join(args, ","), ")")

	_, caller, line, ok := runtime.Caller(1)
	if ok {
		tr := regexp.MustCompile(`(/([^/]*))+$`)
		matches := tr.FindStringSubmatch(caller)
		log.Println(fmt.Sprint(matches[2], ":", line), message)
	} else {
		log.Println(message)
	}
}

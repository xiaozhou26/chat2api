package logx

import (
	"bytes"
	"fmt"
	"strings"
)

type TextFormatter struct{}

func (f *TextFormatter) Format(entry *Entry) ([]byte, error) {
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	if entry.HasCaller() {
		_, _ = fmt.Fprintf(b, "Func %v():%d\n", entry.Caller.Function, entry.Caller.Line)
	}

	_, _ = fmt.Fprintf(b, "%s %s", entry.Time.Format("2006-01-02 15:04:05:06"), paddedLevel(entry.Level.String()))

	tagText := "-"
	if tag, ok := entry.Data[TagKey]; ok {
		tagText = strings.ToUpper(fmt.Sprintf("%v", tag))
		delete(entry.Data, TagKey)
	}
	_, _ = fmt.Fprintf(b, "%-8s ", tagText)

	_, _ = fmt.Fprintf(b, " %s", entry.Message)

	if schoolId, ok := entry.Data[SchoolIDKey]; ok {
		_, _ = fmt.Fprintf(b, "[%v]\t", schoolId)
		delete(entry.Data, SchoolIDKey)
	}

	{
		userId, uidOk := entry.Data[UserIDKey]
		username, unameOk := entry.Data[UsernameKey]
		if uidOk && unameOk {
			_, _ = fmt.Fprintf(b, "[%v(%v)]\t", username, userId)
			delete(entry.Data, UserIDKey)
			delete(entry.Data, UsernameKey)
		}
	}

	if gUuid, ok := entry.Data[GuuIDKey]; ok {
		_, _ = fmt.Fprintf(b, "%s\t", gUuid)
		delete(entry.Data, GuuIDKey)
	}

	if traceId, ok := entry.Data[TraceIdKey]; ok {
		_, _ = fmt.Fprintf(b, " trace=%v", traceId)
		delete(entry.Data, TraceIdKey)
	}

	if stack, ok := entry.Data[StackKey]; ok {
		_, _ = fmt.Fprintf(b, "%s\t", stack)
		delete(entry.Data, StackKey)
	}

	if len(entry.Data) > 0 {
		writeField := func(key string) {
			if value, ok := entry.Data[key]; ok {
				_, _ = fmt.Fprintf(b, " %s=%v", key, value)
				delete(entry.Data, key)
			}
		}
		writeField("client")
		writeField("method")
		writeField("status")
		writeField("latency")
		writeField("path")
		for key, value := range entry.Data {
			_, _ = fmt.Fprintf(b, " %s=%v", key, value)
		}
	}

	b.WriteByte('\n')
	return b.Bytes(), nil
}

func paddedLevel(level string) string {
	level = strings.ToUpper(level)
	for len(level) < 7 {
		level += " "
	}
	return level
}

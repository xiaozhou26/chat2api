package logx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
)

type (
	Logger = logrus.Logger
	Entry  = logrus.Entry
	Hook   = logrus.Hook
	Level  = logrus.Level
	Fields = logrus.Fields
)

func SetLevel(level Level) {
	logrus.SetLevel(level)
}

func SetFormatter(format logrus.Formatter) {
	logrus.SetFormatter(format)
}

var (
	outputMu   sync.Mutex
	outputFile *os.File
)

func Configure(levelText string, logPath string, logFile string) error {
	levelText = strings.TrimSpace(strings.ToLower(levelText))
	if levelText == "" {
		levelText = "debug"
	}
	level, err := logrus.ParseLevel(levelText)
	if err != nil {
		return fmt.Errorf("parse log level %q: %w", levelText, err)
	}
	logrus.SetLevel(level)
	return SetFileOutput(logPath, logFile)
}

func SetFileOutput(logPath string, logFile string) error {
	outputMu.Lock()
	defer outputMu.Unlock()

	var nextFile *os.File
	logFile = strings.TrimSpace(logFile)
	if logFile != "" {
		path := logFile
		if !filepath.IsAbs(path) {
			logPath = strings.TrimSpace(logPath)
			if logPath == "" {
				logPath = "logs"
			}
			path = filepath.Join(logPath, logFile)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return err
		}
		nextFile = file
	}

	if nextFile == nil {
		logrus.SetOutput(stripANSIWriter{w: os.Stdout})
	} else {
		logrus.SetOutput(stripANSIWriter{w: io.MultiWriter(os.Stdout, nextFile)})
	}
	if outputFile != nil {
		_ = outputFile.Close()
	}
	outputFile = nextFile
	return nil
}

func CloseOutput() {
	outputMu.Lock()
	defer outputMu.Unlock()
	logrus.SetOutput(stripANSIWriter{w: os.Stdout})
	if outputFile != nil {
		_ = outputFile.Close()
		outputFile = nil
	}
}

func init() {
	logrus.SetOutput(stripANSIWriter{w: os.Stdout})
}

type stripANSIWriter struct {
	w io.Writer
}

func (s stripANSIWriter) Write(p []byte) (int, error) {
	cleaned := stripANSI(p)
	_, err := s.w.Write(cleaned)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func stripANSI(in []byte) []byte {
	if !bytes.Contains(in, []byte{27, '['}) {
		return in
	}
	out := make([]byte, 0, len(in))
	for i := 0; i < len(in); i++ {
		if in[i] == 27 && i+1 < len(in) && in[i+1] == '[' {
			i += 2
			for i < len(in) {
				b := in[i]
				if b >= 0x40 && b <= 0x7e {
					break
				}
				i++
			}
			continue
		}
		out = append(out, in[i])
	}
	return out
}

func AddHook(hook Hook) {
	logrus.AddHook(hook)
}

const (
	TraceIdKey  = "trace"
	UserIDKey   = "uid"
	GuuIDKey    = "guid"
	SchoolIDKey = "school"
	UsernameKey = "username"
	TagKey      = "tag"
	StackKey    = "stack"
)

type (
	traceIdKey  struct{}
	userIDKey   struct{}
	guuIDKey    struct{}
	usernameKey struct{}
	schoolIdKey struct{}
	tagKey      struct{}
	stackKey    struct{}
	requestKey  struct{}
	responseKey struct{}
	diff1Key    struct{}
	diff2Key    struct{}
	actionKey   struct{}
)

func TraceIdContext(ctx context.Context, traceId string) context.Context {
	return context.WithValue(ctx, traceIdKey{}, traceId)
}

func FromTraceIdContext(ctx context.Context) string {
	if v := ctx.Value(traceIdKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func UserIDContext(ctx context.Context, userId int) context.Context {
	return context.WithValue(ctx, userIDKey{}, userId)
}

func FromUserIDContext(ctx context.Context) int {
	if v := ctx.Value(userIDKey{}); v != nil {
		if s, ok := v.(int); ok {
			return s
		}
	}
	return 0
}

func GuuIDContext(ctx context.Context, userId string) context.Context {
	return context.WithValue(ctx, guuIDKey{}, userId)
}

func FromGuuIDContext(ctx context.Context) string {
	if v := ctx.Value(guuIDKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func SchoolIDContext(ctx context.Context, userId int) context.Context {
	return context.WithValue(ctx, schoolIdKey{}, userId)
}

func FromSchoolIDContext(ctx context.Context) int {
	if v := ctx.Value(schoolIdKey{}); v != nil {
		if s, ok := v.(int); ok {
			return s
		}
	}
	return 0
}

func UsernameContext(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, usernameKey{}, username)
}

func FromUsernameContext(ctx context.Context) string {
	if v := ctx.Value(usernameKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func TagContext(ctx context.Context, tag string) context.Context {
	return context.WithValue(ctx, tagKey{}, tag)
}

func FromTagContext(ctx context.Context) string {
	if v := ctx.Value(tagKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func ActionContext(ctx context.Context, action string) context.Context {
	return context.WithValue(ctx, actionKey{}, action)
}

func FromActionContext(ctx context.Context) string {
	if v := ctx.Value(actionKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func StackContext(ctx context.Context, stack error) context.Context {
	return context.WithValue(ctx, stackKey{}, stack)
}

func FromStackContext(ctx context.Context) error {
	if v := ctx.Value(tagKey{}); v != nil {
		if e, ok := v.(error); ok {
			return e
		}
	}
	return nil
}

func RequestContext(ctx context.Context, data []byte) context.Context {
	return context.WithValue(ctx, requestKey{}, string(data))
}

func FromRequestContext(ctx context.Context) string {
	if v := ctx.Value(requestKey{}); v != "" {
		if e, ok := v.(string); ok {
			return e
		}
	}
	return ""
}

func ResponseContext(ctx context.Context, data any) context.Context {
	bts, _ := json.Marshal(&data)
	return context.WithValue(ctx, responseKey{}, string(bts))
}

func FromResponseContext(ctx context.Context) string {
	if v := ctx.Value(responseKey{}); v != "" {
		if e, ok := v.(string); ok {
			return e
		}
	}
	return ""
}

func DiffContext(ctx context.Context, data1, data2 any) context.Context {
	ctx = context.WithValue(ctx, diff1Key{}, data1)
	ctx = context.WithValue(ctx, diff2Key{}, data2)
	return ctx
}

func FromDiffContext(ctx context.Context) (any, any) {
	return ctx.Value(diff1Key{}), ctx.Value(diff2Key{})
}

func WithContext(ctx context.Context) *Entry {
	fields := logrus.Fields{}
	if v := FromTraceIdContext(ctx); v != "" {
		fields[TraceIdKey] = v
	}

	if v := FromUserIDContext(ctx); v != 0 {
		fields[UserIDKey] = v
	}

	if v := FromGuuIDContext(ctx); v != "" {
		fields[GuuIDKey] = v
	}

	if v := FromSchoolIDContext(ctx); v != 0 {
		fields[SchoolIDKey] = v
	}

	if v := FromUsernameContext(ctx); v != "" {
		fields[UsernameKey] = v
	}

	if v := FromTagContext(ctx); v != "" {
		fields[TagKey] = v
	}

	if v := FromStackContext(ctx); v != nil {
		fields[StackKey] = fmt.Sprintf("%+v", v)
	}

	return logrus.WithContext(ctx).WithFields(fields)
}

var (
	Tracef          = logrus.Tracef
	Debugf          = logrus.Debugf
	Infof           = logrus.Infof
	Warnf           = logrus.Warnf
	Errorf          = logrus.Errorf
	Fatalf          = logrus.Fatalf
	Panicf          = logrus.Panicf
	Printf          = logrus.Printf
	SetOutput       = logrus.SetOutput
	SetReportCaller = logrus.SetReportCaller
	StandardLogger  = logrus.StandardLogger
	ParseLevel      = logrus.ParseLevel
)

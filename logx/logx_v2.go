package logx

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var levelColors = map[LogLevel]string{
	DEBUG: "\033[36m", // 青色
	INFO:  "\033[32m", // 绿色
	WARN:  "\033[33m", // 黄色
	ERROR: "\033[31m", // 红色
}

const resetColor = "\033[0m"

type Logger struct {
	mu          sync.Mutex
	level       LogLevel
	consoleOut  bool
	file        *os.File
	fileWriter  *log.Logger
	maxSize     int64
	filePath    string
	currentSize int64
	logChan     chan logEntry  // 用于异步日志处理
	wg          sync.WaitGroup // 等待日志处理完成
}

type logEntry struct {
	level LogLevel
	msg   string
	time  time.Time
}

func (l *Logger) StartWorker() {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		for entry := range l.logChan {
			l.write(entry)
		}
	}()
}

func NewLogger(filePath string, level LogLevel, maxSizeMB int64, consoleOut bool) (*Logger, error) {
	l := &Logger{
		level:      level,
		consoleOut: consoleOut,
		maxSize:    maxSizeMB * 1024 * 1024,
		filePath:   filePath,
		logChan:    make(chan logEntry, 2000), // 异步日志通道
	}
	if err := l.rotate(); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *Logger) rotate() error {
	if l.file != nil {
		l.file.Close()
	}

	dir := filepath.Dir(l.filePath)
	os.MkdirAll(dir, 0755)

	timestamp := time.Now().Format("20060102_150405")
	newPath := fmt.Sprintf("%s.%s.log", l.filePath, timestamp)

	if _, err := os.Stat(l.filePath); err == nil {
		os.Rename(l.filePath, newPath)
	}

	file, err := os.OpenFile(l.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	l.file = file
	l.fileWriter = log.New(io.MultiWriter(file), "", log.LstdFlags) // 创建日志写入器
	l.currentSize = 0
	return nil
}

func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) log(level LogLevel, msg string) {
	if level < l.level {
		return
	}
	l.logChan <- logEntry{level, msg, time.Now()}
}

func levelString(level LogLevel) string {
	switch level {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// 公共方法
func (l *Logger) Debug(msg string) { l.log(DEBUG, msg) }
func (l *Logger) Info(msg string)  { l.log(INFO, msg) }
func (l *Logger) Warn(msg string)  { l.log(WARN, msg) }
func (l *Logger) Error(msg string) { l.log(ERROR, msg) }

func (l *Logger) Close() {
	close(l.logChan) // 关闭日志通道，停止接收新日志
	l.wg.Wait()      // 等待所有日志处理完成
	if l.file != nil {
		l.file.Close()
	}
}

func (l *Logger) write(entry logEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	formatted := fmt.Sprintf("[%s] %s", levelString(entry.level), entry.msg)

	if l.consoleOut {
		color := levelColors[entry.level]
		fmt.Printf("%s%s%s\n", color, formatted, resetColor)
	}

	err := l.fileWriter.Output(3, formatted)
	if err != nil {
		fmt.Fprintf(os.Stderr, "log write error: %v\n", err)
	}

	l.currentSize += int64(len(formatted) + 1)
	if l.currentSize >= l.maxSize {
		_ = l.rotate()
	}
}

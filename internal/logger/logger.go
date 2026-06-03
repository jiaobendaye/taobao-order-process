// Package logger 简单的文件+控制台日志
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu       sync.Mutex
	logFile  *os.File
	multiOut io.Writer
)

// Init 初始化日志，写入可执行文件同目录的 .log 文件
func Init() {
	execPath, _ := os.Executable()
	logPath := filepath.Join(filepath.Dir(execPath), "phonecase-tools.log")

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("[WARN] 无法创建日志文件 %s: %v", logPath, err)
		multiOut = os.Stdout
		return
	}
	logFile = f
	multiOut = io.MultiWriter(os.Stdout, f)
	log.SetOutput(multiOut)
	log.SetFlags(0) // 用自己的时间格式
	Info("日志已启动: %s", logPath)
}

// Close 关闭日志文件
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if logFile != nil {
		logFile.Close()
	}
}

func fmtLog(level, f string, args ...interface{}) string {
	ts := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(f, args...)
	return fmt.Sprintf("[%s] [%s] %s", ts, level, msg)
}

// Info 信息日志
func Info(f string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	log.Println(fmtLog("INFO", f, args...))
}

// Warn 警告日志
func Warn(f string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	log.Println(fmtLog("WARN", f, args...))
}

// Error 错误日志
func Error(f string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	log.Println(fmtLog("ERROR", f, args...))
}

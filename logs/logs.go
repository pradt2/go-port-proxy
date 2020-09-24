package logs

import (
	"github.com/op/go-logging"
	"os"
)

func Init(logLevel int) {
	backend := logging.NewLogBackend(os.Stdout, "", 0)
	formatter := logging.NewBackendFormatter(backend, logging.MustStringFormatter(
		`%{color}%{time:15:04:05.000} %{module} ► %{level:.4s} %{id:03x}%{color:reset} %{message}`,
	))
	leveled := logging.AddModuleLevel(formatter)
	leveled.SetLevel(logging.Level(logLevel), "")
	logging.SetBackend(leveled)
}

func GetLoggerForModule(module string) *logging.Logger {
	return logging.MustGetLogger(module)
}

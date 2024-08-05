package fxdb

import (
	"context"
	"fmt"
	//logger"gotemplate/logger"
logger"github.com/templatedop/fxlogger"
	"github.com/jackc/pgx/v5/tracelog"
)

type testLogger struct {
	log      *logger.Logger
	FuncName string
	//logs []testLog
}

func (l *testLogger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
	//l.log.Debug(fmt.Sprintf("Level: %v, Message: %s, Data: %v, starttime: %v\n", level, msg, data, data["sql"])) // Print log to console
	l.log.Debug(fmt.Sprintf("Level: %v, Message: %s, Data: %v\n", level, msg, data)) // Print log to console

}

package debug

import "log"

type Debugger struct {
	debug bool
}

func NewDebugger(debug bool) *Debugger {
	return &Debugger{debug: debug}
}

func (d *Debugger) DebugPrintf(format string, args ...interface{}) {
	if d.debug {
		log.Printf(format, args...)
	}
}

func (d *Debugger) DebugPrintln(args ...interface{}) {
	if d.debug {
		log.Println(args...)
	}
}

func (d *Debugger) DebugPrint(v ...interface{}) {
	if d.debug {
		log.Println(v...)
	}
}

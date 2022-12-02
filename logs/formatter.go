package logs

import (
	"path"
	"strconv"
)

type LogFormatter interface {
	Format(lm *LogMsg) string
}

var formatterMap = make(map[string]LogFormatter, 4)

// PatternLogFormatter provides a quick format method
// for example:
// tes := &PatternLogFormatter{Pattern: "%F:%n|%w %t>> %m", WhenFormat: "2006-01-02"}
// RegisterFormatter("tes", tes)
// SetGlobalFormatter("tes")
type PatternLogFormatter struct {
	Pattern  		string
	WhenFormat	string
}

func (this *PatternLogFormatter) getWhenFormatter() string {
	s := this.WhenFormat
	if s == "" {
		s = "2006-01-02 15:04:05.123" // default style
	}
	return s
}


// 'w' when, 'm' msg,'f' filename，'F' full path，'n' line number
// 'l' level number, 't' prefix of level type, 'T' full name of level type
func (this *PatternLogFormatter) ToString(lm *LogMsg) string {
	s := []rune(this.Pattern)
	m := map[rune]string{
		'w': lm.When.Format(this.getWhenFormatter()),
		'm': lm.Msg,
		'n': strconv.Itoa(lm.LineNumber),
		'l': strconv.Itoa(lm.Level),
		't': levelPrefix[lm.Level-1],
		'T': levelNames[lm.Level-1],
		'F': lm.FilePath,
	}
	_, m['f'] = path.Split(lm.FilePath)
	res := ""
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '%' {
			if k, ok := m[s[i+1]]; ok {
				res += k
				i++
				continue
			}
		}
		res += string(s[i])
	}
	return res
}

func (this *PatternLogFormatter) Format(lm *LogMsg) string {
	return this.ToString(lm)
}

// RegisterFormatter register an formatter. Usually you should use this to extend your custom formatter
// for example:
// RegisterFormatter("my-fmt", &MyFormatter{})
// logs.SetFormatter(Console, `{"formatter": "my-fmt"}`)
func RegisterFormatter(name string, fmtr LogFormatter) {
	formatterMap[name] = fmtr
}

func GetFormatter(name string) (LogFormatter, bool) {
	res, ok := formatterMap[name]
	return res, ok
}
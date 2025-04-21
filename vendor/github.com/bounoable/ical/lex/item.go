package lex

import "fmt"

// The lexed item types.
const (
	Error = ItemType(iota)
	EOF

	CalendarBegin
	CalendarEnd
	EventBegin
	EventEnd
	AlarmBegin
	AlarmEnd

	Name
	Value
	ParamName
	ParamValue
)

// Item is a lexed item.
type Item struct {
	Type  ItemType
	Value string
}

// ItemType is the type of a lexed item.
type ItemType int

func (it ItemType) String() string {
	switch it {
	case EOF:
		return "<EOF>"
	case CalendarBegin:
		return "<calendar:begin>"
	case CalendarEnd:
		return "<calendar:end>"
	case EventBegin:
		return "<event:begin>"
	case EventEnd:
		return "<event:end>"
	case AlarmBegin:
		return "<alarm:begin>"
	case AlarmEnd:
		return "<alarm:end>"
	case Name:
		return "<contentline:name>"
	case ParamName:
		return "<param:name>"
	case ParamValue:
		return "<param:value>"
	case Value:
		return "<contentline:value>"
	default:
		return "<unknown>"
	}
}

func (i Item) String() string {
	if i.Type == Error {
		return i.Value
	}

	return fmt.Sprintf("%s (%q)", i.Type, i.Value)
}

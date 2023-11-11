package irc

import (
	"errors"
	"fmt"
	"strings"
)

type ircTag struct {
	Key   string
	Value string
}
type ircCommand struct {
	Name string
	Args []string
}
type ircv3Message struct {
	Prefix  string
	Tags    []ircTag
	Command ircCommand
}

var (
	errTagsNoEqualsSignProvided = errors.New("invalid tag. No equals sign provided")
	errEmptyMessage             = errors.New("empty message")
	errInvalidMessage           = errors.New("invalid message")
)

func parseTags(msg string) ([]ircTag, error) {
	var result []ircTag

	for len(msg) > 0 {
		idxEq := strings.IndexRune(msg, '=')
		if idxEq == -1 {
			return result, errTagsNoEqualsSignProvided
		}
		key := msg[:idxEq]
		msg = msg[idxEq+1:]

		idxSemi := strings.IndexRune(msg, ';')
		if idxSemi == -1 {
			idxSemi = len(msg)
		}
		value := msg[:idxSemi]
		result = append(result, ircTag{
			Key:   key,
			Value: value,
		})
		if len(msg[idxSemi:]) == 0 {
			break
		}
		msg = msg[idxSemi+1:]
	}

	return result, nil
}

var (
	errTagsWithoutMessageBody = errors.New("tags without proper message body")
	errNoCommandSeparator     = errors.New("message does not contain command separator")
	errNoCommand              = errors.New("message does not contain command")
	errNoArgsButDelimiter     = errors.New("message does not contain argument(s) but has delimiter")
)

func parseIRCv3(msg string) (ircv3Message, error) {
	result := ircv3Message{}
	if len(msg) == 0 {
		return result, errEmptyMessage
	}
	// parse tags
	if msg[0] == '@' {
		idxEndOfTags := strings.IndexRune(msg, ' ')
		if idxEndOfTags == -1 {
			return result, errTagsWithoutMessageBody
		}
		tags, err := parseTags(msg[1:idxEndOfTags])
		if err != nil {
			return result, fmt.Errorf("could not parse tags. %w", err)
		}
		result.Tags = tags
		msg = msg[idxEndOfTags+1:]
	}
	if len(msg) == 0 {
		return result, errInvalidMessage
	}

	// parse prefix
	if msg[0] == ':' {
		msg = msg[1:] // trim the ":"
		idxSpc := strings.IndexRune(msg, ' ')
		if idxSpc == -1 {
			return result, errNoCommandSeparator
		}
		result.Prefix = msg[:idxSpc]
		msg = msg[idxSpc+1:]
	} else {
		// probably Ping or smth like that
		// return result, errors.New("does not contain prefix")
	}

	// parse command
	if len(msg) == 0 {
		return result, errNoCommand
	}
	eom := false
	idxSpc := strings.IndexRune(msg, ' ')
	if idxSpc == -1 {
		idxSpc = len(msg)
		eom = true
	}
	result.Command.Name = msg[:idxSpc]
	if eom {
		return result, nil
	} else {
		msg = msg[idxSpc+1:]
	}

	// parse arguments
	if len(msg) == 0 {
		return result, errNoArgsButDelimiter
	}

	for len(msg) > 0 {
		if msg[0] == ':' { // trailer
			result.Command.Args = append(result.Command.Args, msg[1:])
			return result, nil
		}

		idxSpc := strings.IndexRune(msg, ' ')
		if idxSpc == -1 {
			result.Command.Args = append(result.Command.Args, msg)
			return result, nil
		}
		result.Command.Args = append(result.Command.Args, msg[:idxSpc])
		msg = msg[idxSpc+1:]
	}

	return result, nil
}

package json

import (
	"unsafe"
)

type stringDecoder struct {
}

func newStringDecoder() *stringDecoder {
	return &stringDecoder{}
}

func (d *stringDecoder) decodeStream(s *stream, p uintptr) error {
	bytes, err := d.decodeStreamByte(s)
	if err != nil {
		return err
	}
	**(**string)(unsafe.Pointer(&p)) = *(*string)(unsafe.Pointer(&bytes))
	return nil
}

func (d *stringDecoder) decode(buf []byte, cursor int64, p uintptr) (int64, error) {
	bytes, c, err := d.decodeByte(buf, cursor)
	if err != nil {
		return 0, err
	}
	cursor = c
	**(**string)(unsafe.Pointer(&p)) = *(*string)(unsafe.Pointer(&bytes))
	return cursor, nil
}

var (
	hexToInt = [256]int{
		'0': 0,
		'1': 1,
		'2': 2,
		'3': 3,
		'4': 4,
		'5': 5,
		'6': 6,
		'7': 7,
		'8': 8,
		'9': 9,
		'A': 10,
		'B': 11,
		'C': 12,
		'D': 13,
		'E': 14,
		'F': 15,
		'a': 10,
		'b': 11,
		'c': 12,
		'd': 13,
		'e': 14,
		'f': 15,
	}
)

func unicodeToRune(code []byte) rune {
	sum := 0
	for i := 0; i < len(code); i++ {
		sum += hexToInt[code[i]] << (uint(len(code)-i-1) * 4)
	}
	return rune(sum)
}

func decodeEscapeString(s *stream) error {
	s.cursor++
RETRY:
	switch s.buf[s.cursor] {
	case '"':
		s.buf[s.cursor] = '"'
	case '\\':
		s.buf[s.cursor] = '\\'
	case '/':
		s.buf[s.cursor] = '/'
	case 'b':
		s.buf[s.cursor] = '\b'
	case 'f':
		s.buf[s.cursor] = '\f'
	case 'n':
		s.buf[s.cursor] = '\n'
	case 'r':
		s.buf[s.cursor] = '\r'
	case 't':
		s.buf[s.cursor] = '\t'
	case 'u':
		if s.cursor+5 >= s.length {
			if !s.read() {
				return errInvalidCharacter(s.char(), "escaped string", s.totalOffset())
			}
		}
		code := unicodeToRune(s.buf[s.cursor+1 : s.cursor+5])
		unicode := []byte(string(code))
		s.buf = append(append(s.buf[:s.cursor-1], unicode...), s.buf[s.cursor+5:]...)
		s.cursor--
		return nil
	case nul:
		if !s.read() {
			return errInvalidCharacter(s.char(), "escaped string", s.totalOffset())
		}
		goto RETRY
	default:
		return errUnexpectedEndOfJSON("string", s.totalOffset())
	}
	s.buf = append(s.buf[:s.cursor-1], s.buf[s.cursor:]...)
	s.cursor--
	return nil
}

func stringBytes(s *stream) ([]byte, error) {
	s.cursor++
	start := s.cursor
	for {
		switch s.char() {
		case '\\':
			s.cursor++
		case '"':
			literal := s.buf[start:s.cursor]
			s.cursor++
			s.reset()
			return literal, nil
		case nul:
			if s.read() {
				continue
			}
			goto ERROR
		}
		s.cursor++
	}
ERROR:
	return nil, errUnexpectedEndOfJSON("string", s.totalOffset())
}

func nullBytes(s *stream) error {
	if s.cursor+3 >= s.length {
		if !s.read() {
			return errInvalidCharacter(s.char(), "null", s.totalOffset())
		}
	}
	s.cursor++
	if s.char() != 'u' {
		return errInvalidCharacter(s.char(), "null", s.totalOffset())
	}
	s.cursor++
	if s.char() != 'l' {
		return errInvalidCharacter(s.char(), "null", s.totalOffset())
	}
	s.cursor++
	if s.char() != 'l' {
		return errInvalidCharacter(s.char(), "null", s.totalOffset())
	}
	s.cursor++
	return nil
}

func (d *stringDecoder) decodeStreamByte(s *stream) ([]byte, error) {
	for {
		switch s.char() {
		case ' ', '\n', '\t', '\r':
			s.cursor++
			continue
		case '"':
			return stringBytes(s)
		case 'n':
			if err := nullBytes(s); err != nil {
				return nil, err
			}
			return []byte{}, nil
		case nul:
			if s.read() {
				continue
			}
		}
		break
	}
	return nil, errNotAtBeginningOfValue(s.totalOffset())
}

func (d *stringDecoder) decodeByte(buf []byte, cursor int64) ([]byte, int64, error) {
	for {
		switch buf[cursor] {
		case ' ', '\n', '\t', '\r':
			cursor++
		case '"':
			cursor++
			start := cursor
			for {
				switch buf[cursor] {
				case '\\':
					cursor++
					switch buf[cursor] {
					case '"':
						buf[cursor] = '"'
						buf = append(buf[:cursor-1], buf[cursor:]...)
					case '\\':
						buf[cursor] = '\\'
						buf = append(buf[:cursor-1], buf[cursor:]...)
					case '/':
						buf[cursor] = '/'
						buf = append(buf[:cursor-1], buf[cursor:]...)
					case 'b':
						buf[cursor] = '\b'
						buf = append(buf[:cursor-1], buf[cursor:]...)
					case 'f':
						buf[cursor] = '\f'
						buf = append(buf[:cursor-1], buf[cursor:]...)
					case 'n':
						buf[cursor] = '\n'
						buf = append(buf[:cursor-1], buf[cursor:]...)
					case 'r':
						buf[cursor] = '\r'
						buf = append(buf[:cursor-1], buf[cursor:]...)
					case 't':
						buf[cursor] = '\t'
						buf = append(buf[:cursor-1], buf[cursor:]...)
					case 'u':
						buflen := int64(len(buf))
						if cursor+5 >= buflen {
							return nil, 0, errUnexpectedEndOfJSON("escaped string", cursor)
						}
						code := unicodeToRune(buf[cursor+1 : cursor+5])
						unicode := []byte(string(code))
						buf = append(append(buf[:cursor-1], unicode...), buf[cursor+5:]...)
					default:
						return nil, 0, errUnexpectedEndOfJSON("escaped string", cursor)
					}
					continue
				case '"':
					literal := buf[start:cursor]
					cursor++
					return literal, cursor, nil
				case nul:
					return nil, 0, errUnexpectedEndOfJSON("string", cursor)
				}
				cursor++
			}
			return nil, 0, errUnexpectedEndOfJSON("string", cursor)
		case 'n':
			buflen := int64(len(buf))
			if cursor+3 >= buflen {
				return nil, 0, errUnexpectedEndOfJSON("null", cursor)
			}
			if buf[cursor+1] != 'u' {
				return nil, 0, errInvalidCharacter(buf[cursor+1], "null", cursor)
			}
			if buf[cursor+2] != 'l' {
				return nil, 0, errInvalidCharacter(buf[cursor+2], "null", cursor)
			}
			if buf[cursor+3] != 'l' {
				return nil, 0, errInvalidCharacter(buf[cursor+3], "null", cursor)
			}
			cursor += 4
			return []byte{}, cursor, nil
		default:
			goto ERROR
		}
	}
ERROR:
	return nil, 0, errNotAtBeginningOfValue(cursor)
}

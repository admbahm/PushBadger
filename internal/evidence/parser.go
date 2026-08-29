package evidence

import (
	"fmt"
	"strconv"
	"unicode/utf8"
)

// number retains the exact JSON token. It must never be converted through
// float64: schema integer checks operate on the represented decimal value.
type number struct {
	raw string
}

type object map[string]any
type array []any

type jsonParser struct {
	data []byte
	pos  int
}

func parse(data []byte) (any, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("parse error: input is not valid UTF-8")
	}

	parser := jsonParser{data: data}
	value, err := parser.parseValue("$")
	if err != nil {
		return nil, err
	}
	parser.skipWhitespace()
	if parser.pos != len(parser.data) {
		return nil, fmt.Errorf("parse error: unexpected trailing token at byte %d", parser.pos)
	}
	return value, nil
}

func (p *jsonParser) parseValue(path string) (any, error) {
	p.skipWhitespace()
	if p.pos == len(p.data) {
		return nil, fmt.Errorf("parse error at %s: unexpected end of input", path)
	}

	switch p.data[p.pos] {
	case '{':
		return p.parseObject(path)
	case '[':
		return p.parseArray(path)
	case '"':
		return p.parseString(path)
	case 't':
		if p.consumeLiteral("true") {
			return true, nil
		}
	case 'f':
		if p.consumeLiteral("false") {
			return false, nil
		}
	case 'n':
		if p.consumeLiteral("null") {
			return nil, nil
		}
	default:
		if p.data[p.pos] == '-' || isDigit(p.data[p.pos]) {
			return p.parseNumber(path)
		}
	}
	return nil, fmt.Errorf("parse error at %s: unexpected token at byte %d", path, p.pos)
}

func (p *jsonParser) parseObject(path string) (object, error) {
	p.pos++
	result := make(object)
	p.skipWhitespace()
	if p.consumeByte('}') {
		return result, nil
	}

	for {
		p.skipWhitespace()
		if p.pos == len(p.data) || p.data[p.pos] != '"' {
			return nil, fmt.Errorf("parse error at %s: object key must be a string", path)
		}
		key, err := p.parseString(path)
		if err != nil {
			return nil, err
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("parse error at %s: duplicate object key %q", path, key)
		}

		p.skipWhitespace()
		if !p.consumeByte(':') {
			return nil, fmt.Errorf("parse error at %s: expected ':' after object key", path)
		}
		value, err := p.parseValue(propertyPath(path, key))
		if err != nil {
			return nil, err
		}
		result[key] = value

		p.skipWhitespace()
		if p.consumeByte('}') {
			return result, nil
		}
		if !p.consumeByte(',') {
			return nil, fmt.Errorf("parse error at %s: expected ',' or '}'", path)
		}
	}
}

func (p *jsonParser) parseArray(path string) (array, error) {
	p.pos++
	var result array
	p.skipWhitespace()
	if p.consumeByte(']') {
		return result, nil
	}

	for index := 0; ; index++ {
		value, err := p.parseValue(fmt.Sprintf("%s[%d]", path, index))
		if err != nil {
			return nil, err
		}
		result = append(result, value)

		p.skipWhitespace()
		if p.consumeByte(']') {
			return result, nil
		}
		if !p.consumeByte(',') {
			return nil, fmt.Errorf("parse error at %s: expected ',' or ']'", path)
		}
	}
}

func (p *jsonParser) parseString(path string) (string, error) {
	p.pos++
	decoded := make([]byte, 0)
	for p.pos < len(p.data) {
		current := p.data[p.pos]
		p.pos++
		switch current {
		case '"':
			return string(decoded), nil
		case '\\':
			if p.pos == len(p.data) {
				return "", fmt.Errorf("parse error at %s: incomplete string escape", path)
			}
			escape := p.data[p.pos]
			p.pos++
			switch escape {
			case '"', '\\', '/':
				decoded = append(decoded, escape)
			case 'b':
				decoded = append(decoded, '\b')
			case 'f':
				decoded = append(decoded, '\f')
			case 'n':
				decoded = append(decoded, '\n')
			case 'r':
				decoded = append(decoded, '\r')
			case 't':
				decoded = append(decoded, '\t')
			case 'u':
				unit, ok := p.consumeHexCodeUnit()
				if !ok {
					return "", fmt.Errorf("parse error at %s: invalid Unicode escape", path)
				}
				if isHighSurrogate(unit) && p.pos+6 <= len(p.data) && p.data[p.pos] == '\\' && p.data[p.pos+1] == 'u' {
					low, lowOK := readHexCodeUnit(p.data[p.pos+2 : p.pos+6])
					if lowOK && isLowSurrogate(low) {
						p.pos += 6
						combined := rune(0x10000 + (uint32(unit-0xd800) << 10) + uint32(low-0xdc00))
						decoded = utf8.AppendRune(decoded, combined)
						continue
					}
				}
				decoded = appendCodeUnit(decoded, unit)
			default:
				return "", fmt.Errorf("parse error at %s: invalid string escape", path)
			}
		default:
			if current < 0x20 {
				return "", fmt.Errorf("parse error at %s: unescaped control character in string", path)
			}
			decoded = append(decoded, current)
		}
	}
	return "", fmt.Errorf("parse error at %s: unterminated string", path)
}

func (p *jsonParser) parseNumber(path string) (number, error) {
	start := p.pos
	if p.consumeByte('-') && p.pos == len(p.data) {
		return number{}, fmt.Errorf("parse error at %s: invalid number", path)
	}

	if p.consumeByte('0') {
		if p.pos < len(p.data) && isDigit(p.data[p.pos]) {
			return number{}, fmt.Errorf("parse error at %s: invalid leading zero", path)
		}
	} else {
		if p.pos == len(p.data) || p.data[p.pos] < '1' || p.data[p.pos] > '9' {
			return number{}, fmt.Errorf("parse error at %s: invalid number", path)
		}
		for p.pos < len(p.data) && isDigit(p.data[p.pos]) {
			p.pos++
		}
	}

	if p.consumeByte('.') {
		fractionStart := p.pos
		for p.pos < len(p.data) && isDigit(p.data[p.pos]) {
			p.pos++
		}
		if p.pos == fractionStart {
			return number{}, fmt.Errorf("parse error at %s: number fraction requires a digit", path)
		}
	}
	if p.pos < len(p.data) && (p.data[p.pos] == 'e' || p.data[p.pos] == 'E') {
		p.pos++
		if p.pos < len(p.data) && (p.data[p.pos] == '+' || p.data[p.pos] == '-') {
			p.pos++
		}
		exponentStart := p.pos
		for p.pos < len(p.data) && isDigit(p.data[p.pos]) {
			p.pos++
		}
		if p.pos == exponentStart {
			return number{}, fmt.Errorf("parse error at %s: number exponent requires a digit", path)
		}
	}
	return number{raw: string(p.data[start:p.pos])}, nil
}

func (p *jsonParser) consumeHexCodeUnit() (uint16, bool) {
	if p.pos+4 > len(p.data) {
		return 0, false
	}
	unit, ok := readHexCodeUnit(p.data[p.pos : p.pos+4])
	if ok {
		p.pos += 4
	}
	return unit, ok
}

func readHexCodeUnit(data []byte) (uint16, bool) {
	if len(data) != 4 {
		return 0, false
	}
	var result uint16
	for _, digit := range data {
		result <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			result += uint16(digit - '0')
		case digit >= 'a' && digit <= 'f':
			result += uint16(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			result += uint16(digit-'A') + 10
		default:
			return 0, false
		}
	}
	return result, true
}

// Python's JSON decoder retains lone UTF-16 surrogate code points. A Go string
// cannot encode those as UTF-8 runes, so use their collision-free CESU-8 byte
// form internally. Raw input is valid UTF-8, so no submitted scalar can collide
// with this representation; valid surrogate pairs are decoded normally.
func appendCodeUnit(dst []byte, unit uint16) []byte {
	if isHighSurrogate(unit) || isLowSurrogate(unit) {
		return append(dst,
			byte(0xe0|unit>>12),
			byte(0x80|(unit>>6)&0x3f),
			byte(0x80|unit&0x3f),
		)
	}
	return utf8.AppendRune(dst, rune(unit))
}

func decodedCharacterCount(value string) int {
	count := 0
	for index := 0; index < len(value); count++ {
		_, size := utf8.DecodeRuneInString(value[index:])
		if size > 1 {
			index += size
			continue
		}
		// Lone surrogates use the internal CESU-8 form produced above. Count
		// each encoded UTF-16 code unit as one Python string character.
		if index+3 <= len(value) && value[index] == 0xed &&
			value[index+1] >= 0xa0 && value[index+1] <= 0xbf &&
			value[index+2] >= 0x80 && value[index+2] <= 0xbf {
			index += 3
			continue
		}
		index++
	}
	return count
}

func isHighSurrogate(unit uint16) bool {
	return unit >= 0xd800 && unit <= 0xdbff
}

func isLowSurrogate(unit uint16) bool {
	return unit >= 0xdc00 && unit <= 0xdfff
}

func (p *jsonParser) consumeLiteral(literal string) bool {
	if len(p.data)-p.pos < len(literal) || string(p.data[p.pos:p.pos+len(literal)]) != literal {
		return false
	}
	p.pos += len(literal)
	return true
}

func (p *jsonParser) consumeByte(want byte) bool {
	if p.pos == len(p.data) || p.data[p.pos] != want {
		return false
	}
	p.pos++
	return true
}

func (p *jsonParser) skipWhitespace() {
	for p.pos < len(p.data) {
		switch p.data[p.pos] {
		case ' ', '\t', '\r', '\n':
			p.pos++
		default:
			return
		}
	}
}

func isDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func propertyPath(parent, key string) string {
	return parent + "[" + strconv.Quote(key) + "]"
}

package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

func Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func UnmarshalJsonStr(data string, v any) error {
	return json.Unmarshal(StringToByteSlice(data), v)
}

func DecodeJson(reader io.Reader, v any) error {
	return json.NewDecoder(reader).Decode(v)
}

// DecodeJsonStrict 严格解码：拒绝未知字段、拒绝结构体之后存在尾随 JSON、拒绝非法 JSON。
// 供企业身份 X-AI-Context（文档 7.3，严格 Schema）等需要严格校验的协议使用。
// 返回错误时，v 不被信任。
func DecodeJsonStrict(data []byte, v any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}
	// 结构体之后不允许再有任何非空白 JSON（尾随对象/数组/标量都拒绝）。
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			// Token() 读到下一个合法值但非 EOF：存在尾随 JSON。
			return errors.New("unexpected trailing JSON after value")
		}
		return err
	}
	return nil
}

func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func IndentJson(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	if err := json.Indent(&buffer, data, "", "  "); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func GetJsonType(data json.RawMessage) string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return "unknown"
	}
	firstChar := trimmed[0]
	switch firstChar {
	case '{':
		return "object"
	case '[':
		return "array"
	case '"':
		return "string"
	case 't', 'f':
		return "boolean"
	case 'n':
		return "null"
	default:
		return "number"
	}
}

// JsonRawMessageToString returns JSON strings as their decoded value and other JSON values as raw text.
func JsonRawMessageToString(data json.RawMessage) string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	if trimmed[0] != '"' {
		return string(trimmed)
	}
	var value string
	if err := Unmarshal(trimmed, &value); err != nil {
		return string(trimmed)
	}
	return value
}

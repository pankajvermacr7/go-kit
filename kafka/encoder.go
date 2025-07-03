package kafka

import "encoding/json"

type Encoder interface {
	Encode(v interface{}) ([]byte, error)
	Decode(data []byte, v interface{}) error
}

type JSONEncoder struct{}

func (j JSONEncoder) Encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (j JSONEncoder) Decode(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

type StringEncoder struct{}

func (s StringEncoder) Encode(v interface{}) ([]byte, error) {
	return []byte(v.(string)), nil
}

func (s StringEncoder) Decode(data []byte, v interface{}) error {
	*(v.(*string)) = string(data)
	return nil
}

package wstchannel

// Simple helper functions for manipulating JSON

import (
	"encoding/json"
	"fmt"
	"strings"
)

// UnmarshalJsonString unmarshals a golang value from a single properly formatted JSON value
// in js. Additional characters in js beyond a single json value are not allowed, and will produce an error
// (note that a json dict or a json array are single values).
func UnmarshalJsonString(js string, v interface{}) error {
	return json.Unmarshal([]byte(js), v)
}

// ParseJsonValueInString extracts the properly formatted JSON value in
// of the string js (note that a json dict or a json array are single values).
//
// On success:
//    raw = a slice of bytes representing the marshalled json, which may be later unmarshalled into a custom
//          object, or into a generic object with DecodeGenericJsonRawMessage.
//    nb = The number of bytes in js that were consumed by the valid JSON value.
//    err = nil
//
// On error (e.g., a syntax error):
//    raw = nil
//    nb = A best guess at the number of bytes in js that were inspected before an error occurred. Generally this includes the offending character
//          so it may make sense to back up one character before displaying the error. However, some errors such as unterminated quotes will
//          return nb == 0 so be prepared for that.
//    err = a description of the error.
func ParseJsonValueInString(js string) (raw json.RawMessage, nb int, err error) {
	raw, nb, err = ParseNextJsonValueInString(js)
	if err == nil {
		if nb < len(js) {
			err = fmt.Errorf("Unexpected character(s) after valid JSON value: \"%s\"", js[nb:])
			raw = nil
		}
	}

	return raw, nb, err
}

// ParseNextJsonValueInString extracts the properly formatted JSON value at the very beginning
// of the string js. It is not an error to have additional characters in js which may or may not
// be valid json, as long as a single valid json value can be decoded without error (note that
// a json dict or a json array are single values).
//
// On success:
//    raw = a slice of bytes representing the marshalled json, which may be later unmarshalled into a custom
//          object, or into a generic object with DecodeGenericJsonRawMessage.
//    nb = The number of bytes in js that were consumed by the valid JSON value.
//    err = nil
//
// On error (e.g., a syntax error):
//    raw = nil
//    nb = A best guess at the number of bytes in js that were inspected before an error occurred. Generally this includes the offending character
//          so it may make sense to back up one character before displaying the error. However, some errors such as unterminated quotes will
//          return nb == 0 so be prepared for that.
//    err = a description of the error.
func ParseNextJsonValueInString(js string) (raw json.RawMessage, nb int, err error) {
	jsonReader := strings.NewReader(js)
	decodeStream := json.NewDecoder(jsonReader)
	err = decodeStream.Decode(&raw)
	nb = int(decodeStream.InputOffset())
	if err != nil {
		if jsonError, ok := err.(*json.SyntaxError); ok {
			nb = int(jsonError.Offset)
		}
	}

	return raw, nb, err
}

// DecodeGenericJsonRawMessage unmarshals a json.RawMessage into a generic interface{}, which may
// be an int, a float, a string, a bool, a slice of similar generics, or a map from a string to
// a similar generic.
func DecodeGenericJsonRawMessage(raw json.RawMessage) (interface{}, error) {
	var genValue interface{}
	var err error
	if raw != nil {
		err = json.Unmarshal(raw, &genValue)
	}
	return genValue, err
}

// DecodeGenericJsonString unmarshals a JSON value incoded in a string into a generic interface{}, which may
// be an int, a float, a string, a bool, a slice of similar generics, or a map from a string to
// a similar generic.
func DecodeGenericJsonString(js string) (interface{}, error) {
	return DecodeGenericJsonRawMessage([]byte(js))
}

// ToPrettyJsonString marshalls an arbitrary generic into a json string with indentation.
// The generic may be a struct or an object with a custom marshaller, or any of the primitive types
// returned by DecodeGenericJsonRawMessage
func ToPrettyJsonString(v interface{}) (string, error) {

	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	err := enc.Encode(v)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

// ToCompactJsonString marshalls an arbitrary generic into a compact json string with no indentation or newlines.
// The generic may be a struct or an object with a custom marshaller, or any of the primitive types
// returned by DecodeGenericJsonRawMessage
func ToCompactJsonString(v interface{}) (string, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	err := enc.Encode(v)
	if err != nil {
		return "", err
	}
	return b.String(), nil
}

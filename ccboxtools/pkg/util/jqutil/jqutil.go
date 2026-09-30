// Package jqutil evaluates jq selectors over decoded documents with gojq.
package jqutil

import (
	"fmt"
	"reflect"

	"github.com/itchyny/gojq"
)

// SelectStr is the selector's single string result on the decoded document.
func SelectStr(selector string, doc any) (string, error) {
	res, err := selectOne(selector, doc)
	if err != nil {
		return "", err
	}
	s, ok := res.(string)
	if !ok {
		return "", fmt.Errorf("selector %q: result is not a string: %T", selector, res)
	}
	return s, nil
}

// SelectFields selects each exported string field's jq selector on doc — empty
// selectors skip — swapping the selector for its single string result. Panics
// on a non-string field: select structs are all-string by construction.
func SelectFields[T any](sel T, doc any) (T, error) {
	value := reflect.ValueOf(&sel).Elem()
	for i := range value.NumField() {
		field := value.Field(i)
		if !field.CanSet() { // unexported
			continue
		}
		if field.Kind() != reflect.String {
			panic(fmt.Sprintf("SelectFields: %s is not a string field", value.Type().Field(i).Name))
		}
		if field.String() == "" {
			continue
		}
		s, err := SelectStr(field.String(), doc)
		if err != nil {
			return sel, err
		}
		field.SetString(s)
	}
	return sel, nil
}

// selectOne evaluates the selector on the document, requiring exactly one result.
func selectOne(selector string, doc any) (any, error) {
	query, err := gojq.Parse(selector)
	if err != nil {
		return nil, fmt.Errorf("selector %q: %w", selector, err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		return nil, fmt.Errorf("selector %q: %w", selector, err)
	}

	iter := code.Run(doc)
	res, ok := iter.Next()
	if !ok {
		return nil, fmt.Errorf("selector %q: no result", selector)
	}
	if err, ok := res.(error); ok {
		return nil, fmt.Errorf("selector %q: %w", selector, err)
	}
	if _, more := iter.Next(); more {
		return nil, fmt.Errorf("selector %q: more than one result", selector)
	}
	return res, nil
}

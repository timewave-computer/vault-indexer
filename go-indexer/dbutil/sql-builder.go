package dbutil

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

func toDBValue(v reflect.Value) (interface{}, error) {
	if !v.IsValid() {
		return nil, nil
	}
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, nil
		}
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct && v.Type().String() != "time.Time" {
		jsonBytes, err := json.Marshal(v.Interface())
		if err != nil {
			return nil, err
		}
		return string(jsonBytes), nil
	}
	return v.Interface(), nil
}

func BuildInsert(table string, s any) (string, []interface{}, error) {
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()

	var columns []string
	var placeholders []string
	var args []interface{}
	argIndex := 1

	for i := 0; i < v.NumField(); i++ {
		fieldVal := v.Field(i)
		fieldType := t.Field(i)
		column := fieldType.Tag.Get("json")

		// Skip "-" fields
		if column == "-" || column == "" {
			continue
		}

		val, err := toDBValue(fieldVal)
		if err != nil {
			return "", nil, err
		}
		// Include only non-nil values
		if val != nil {
			columns = append(columns, column)
			placeholders = append(placeholders, fmt.Sprintf("$%d", argIndex))
			args = append(args, val)
			argIndex++
		}
	}

	if len(columns) == 0 {
		return "", nil, fmt.Errorf("no fields to insert")
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)
	return query, args, nil
}

func BuildUpdate(table string, s any, whereKeys []string) (string, []interface{}, error) {
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()

	setClauses := []string{}
	whereClauses := []string{}
	args := []interface{}{}
	argIndex := 1

	whereSet := map[string]bool{}
	for _, key := range whereKeys {
		whereSet[key] = true
	}

	for i := 0; i < v.NumField(); i++ {
		fieldVal := v.Field(i)
		fieldType := t.Field(i)
		column := fieldType.Tag.Get("json")

		if column == "-" || column == "" {
			continue
		}

		val, err := toDBValue(fieldVal)
		if err != nil {
			return "", nil, err
		}
		if val == nil {
			continue
		}

		if whereSet[column] {
			whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", column, argIndex))
		} else {
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", column, argIndex))
		}
		args = append(args, val)
		argIndex++
	}

	if len(setClauses) == 0 {
		return "", nil, fmt.Errorf("no fields to update")
	}
	if len(whereClauses) == 0 {
		return "", nil, fmt.Errorf("no WHERE condition provided")
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		table,
		strings.Join(setClauses, ", "),
		strings.Join(whereClauses, " AND "),
	)
	return query, args, nil
}

package dbutil

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/timewave/vault-indexer/go-indexer/database"
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

	// Handle slice of structs
	if v.Kind() == reflect.Slice {
		if v.Len() == 0 {
			return "", nil, fmt.Errorf("empty slice provided")
		}
		// Use the first element of the slice
		v = v.Index(0)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
	}

	if v.Kind() != reflect.Struct {
		return "", nil, fmt.Errorf("expected struct or slice of structs, got %v", v.Kind())
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

	// Handle slice of structs
	if v.Kind() == reflect.Slice {
		if v.Len() == 0 {
			return "", nil, fmt.Errorf("empty slice provided")
		}
		// Use the first element of the slice
		v = v.Index(0)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
	}

	if v.Kind() != reflect.Struct {
		return "", nil, fmt.Errorf("expected struct or slice of structs, got %v", v.Kind())
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
		return "", nil, fmt.Errorf("no WHERE condition provided in update")
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		table,
		strings.Join(setClauses, ", "),
		strings.Join(whereClauses, " AND "),
	)
	return query, args, nil
}

func BuildCleanupQuery(operation interface{}) (string, []interface{}, error) {
	switch op := operation.(type) {
	case database.CleanupOperation:
		switch op.Type {
		case database.CleanupDelete:
			return BuildDeleteWithCustomFilter(op.Table, op.Data, op.Filter)
		case database.CleanupUpdate:
			return BuildUpdateWithCustomFilter(op.Table, op.Data, op.Filter)
		default:
			return "", nil, fmt.Errorf("unsupported cleanup operation type: %v", op.Type)
		}
	default:
		return "", nil, fmt.Errorf("unsupported operation type: %T", operation)
	}
}

// BuildDeleteWithCustomFilter is like BuildDelete but allows custom WHERE field names
func BuildDeleteWithCustomFilter(table string, s any, whereKeys []string) (string, []interface{}, error) {
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return "", nil, fmt.Errorf("expected struct, got %v", v.Kind())
	}

	t := v.Type()
	whereSet := map[string]bool{}
	for _, key := range whereKeys {
		whereSet[key] = true
	}

	var whereClauses []string
	var args []interface{}
	argIndex := 1

	for i := 0; i < v.NumField(); i++ {
		fieldVal := v.Field(i)
		fieldType := t.Field(i)
		column := fieldType.Tag.Get("json")

		if column == "-" || column == "" || !whereSet[column] {
			continue
		}

		val, err := toDBValue(fieldVal)
		if err != nil {
			return "", nil, err
		}
		if val == nil {
			continue
		}

		// Handle special case for position_end_height_where -> position_end_height
		actualColumn := column
		if column == "position_start_height_where" {
			actualColumn = "position_start_height"
		}

		whereClauses = append(whereClauses, fmt.Sprintf("%s > $%d", actualColumn, argIndex))
		args = append(args, val)
		argIndex++
	}

	if len(whereClauses) == 0 {
		return "", nil, fmt.Errorf("no WHERE condition provided in custom delete")
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE %s", table, strings.Join(whereClauses, " AND "))
	return query, args, nil
}

// BuildUpdateWithCustomFilter is like BuildUpdate but allows custom WHERE field names
func BuildUpdateWithCustomFilter(table string, s any, whereKeys []string) (string, []interface{}, error) {
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return "", nil, fmt.Errorf("expected struct, got %v", v.Kind())
	}

	t := v.Type()
	whereSet := map[string]bool{}
	for _, key := range whereKeys {
		whereSet[key] = true
	}

	setClauses := []string{}
	whereClauses := []string{}
	args := []interface{}{}
	argIndex := 1

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

		if whereSet[column] {
			if val != nil {
				// Handle special case for position_end_height_where -> position_end_height
				actualColumn := column
				if column == "position_end_height_where" {
					actualColumn = "position_end_height"
				}
				whereClauses = append(whereClauses, fmt.Sprintf("%s > $%d", actualColumn, argIndex))
				args = append(args, val)
				argIndex++
			}
		} else {
			// For SET clause, include fields even if they're null (to set to NULL)
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", column, argIndex))
			args = append(args, val)
			argIndex++
		}
	}

	if len(setClauses) == 0 {
		return "", nil, fmt.Errorf("no fields to update")
	}
	if len(whereClauses) == 0 {
		return "", nil, fmt.Errorf("no WHERE condition provided in custom update")
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		table,
		strings.Join(setClauses, ", "),
		strings.Join(whereClauses, " AND "),
	)
	return query, args, nil
}

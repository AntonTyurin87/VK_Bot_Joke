package repository

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/Masterminds/squirrel"
)

// Sqlizer ...
type Sqlizer interface {
	ToSql() (sql string, args []interface{}, err error)
}

type toSQLFn func() (sqlStr string, args []interface{}, err error)

func (fn toSQLFn) ToSql() (sqlStr string, args []interface{}, err error) { return fn() }

// Select упрощенная версия без mapToStruct
func Select[TSlice ~[]*T, T any](
	ctx context.Context,
	e interface {
		Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	},
	dest *TSlice,
	query string,
	args ...interface{},
) error {
	if dest == nil {
		return fmt.Errorf("dest cannot be nil")
	}

	rows, err := e.Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	return scanRowsToSlice(rows, dest)
}

func Selectx[TSlice ~[]*T, T any](
	ctx context.Context,
	e interface {
		Query(ctx context.Context, sql string, args ...interface{}) (*sql.Rows, error)
	},
	dest *TSlice,
	sqlizer Sqlizer,
) error {
	sqlizer = ReplacePlaceholders(sqlizer)
	stmt, args, err := sqlizer.ToSql()
	if err != nil {
		return err
	}

	return Select[TSlice](ctx, e, dest, stmt, args...)
}

func ReplacePlaceholders(sqlizer Sqlizer) Sqlizer {
	var (
		sqlStr string
		args   []interface{}
		err    error
	)

	fn := toSQLFn(func() (string, []interface{}, error) { return sqlStr, args, err })

	sqlStr, args, err = sqlizer.ToSql()
	if err != nil {
		return fn
	}

	sqlStr, err = squirrel.Dollar.ReplacePlaceholders(sqlStr)

	return fn
}

// scanRowsToSlice сканирует строки в срез структур
func scanRowsToSlice[TSlice ~[]*T, T any](rows *sql.Rows, dest *TSlice) error {
	if rows == nil {
		return fmt.Errorf("rows is nil")
	}

	// Получаем колонки
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("get columns failed: %w", err)
	}

	if len(columns) == 0 {
		*dest = nil
		return nil
	}

	// Кэш для информации о полях
	var fieldCache sync.Map

	// Создаем буфер для сканирования
	scanArgs := make([]interface{}, len(columns))
	for i := range scanArgs {
		scanArgs[i] = new(interface{})
	}

	var results []*T

	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return fmt.Errorf("scan failed: %w", err)
		}

		// Создаем новую структуру
		item := new(T)
		if item == nil {
			return fmt.Errorf("failed to create new instance of type %T", *new(T))
		}

		// Получаем или создаем маппинг полей для этого типа
		var t T
		structType := reflect.TypeOf(t)
		if structType.Kind() != reflect.Struct {
			return fmt.Errorf("type T must be a struct, got %v", structType.Kind())
		}

		// Пытаемся получить из кэша
		cacheKey := structType.String()
		var columnMapping []int
		if cached, ok := fieldCache.Load(cacheKey); ok {
			columnMapping = cached.([]int)
		} else {
			// Создаем маппинг
			columnMapping = make([]int, len(columns))
			for i, col := range columns {
				columnMapping[i] = -1 // По умолчанию не найдено

				// Ищем поле в структуре
				//colLower := strings.ToLower(col)
				for j := 0; j < structType.NumField(); j++ {
					field := structType.Field(j)
					if !field.IsExported() {
						continue
					}

					// Проверяем тег db
					fieldName := field.Name
					if dbTag := field.Tag.Get("db"); dbTag != "" && dbTag != "-" {
						// Берем первую часть тега
						if commaIdx := strings.Index(dbTag, ","); commaIdx != -1 {
							fieldName = dbTag[:commaIdx]
						} else {
							fieldName = dbTag
						}
					}

					// Сравниваем имя поля с именем колонки
					if strings.EqualFold(fieldName, col) {
						columnMapping[i] = j
						break
					}

					// Пробуем преобразовать snake_case в CamelCase и наоборот
					if matchFieldName(fieldName, col) {
						columnMapping[i] = j
						break
					}
				}
			}
			fieldCache.Store(cacheKey, columnMapping)
		}

		// Заполняем поля структуры
		v := reflect.ValueOf(item).Elem()
		if !v.IsValid() {
			return fmt.Errorf("invalid value for item")
		}

		for i, fieldIndex := range columnMapping {
			if fieldIndex == -1 {
				continue // Поле не найдено
			}

			valPtr := scanArgs[i].(*interface{})
			if valPtr == nil {
				// NULL значение
				continue
			}

			val := reflect.ValueOf(*valPtr)
			if val.IsValid() {
				field := v.Field(fieldIndex)
				if field.IsValid() && field.CanSet() {
					if err := safeSetFieldValue(field, val); err != nil {
						// Логируем ошибку, но продолжаем обработку
						fieldName := structType.Field(fieldIndex).Name
						colName := columns[i]
						fmt.Printf("Warning: failed to set field %s from column %s: %v\n",
							fieldName, colName, err)
					}
				}
			}
		}

		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows iteration failed: %w", err)
	}

	*dest = results
	return nil
}

// matchFieldName проверяет соответствие имени поля и колонки
func matchFieldName(fieldName, columnName string) bool {
	// Прямое сравнение без учета регистра
	if strings.EqualFold(fieldName, columnName) {
		return true
	}

	// Преобразование snake_case в CamelCase
	snakeToCamel := snakeToCamel(columnName)
	if strings.EqualFold(fieldName, snakeToCamel) {
		return true
	}

	// Преобразование CamelCase в snake_case
	camelToSnake := camelToSnake(fieldName)
	if strings.EqualFold(camelToSnake, columnName) {
		return true
	}

	return false
}

// snakeToCamel преобразует snake_case в CamelCase
func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// camelToSnake преобразует CamelCase в snake_case
func camelToSnake(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result = append(result, '_')
		}
		result = append(result, unicode.ToLower(r))
	}
	return string(result)
}

// safeSetFieldValue безопасно устанавливает значение поля
func safeSetFieldValue(field reflect.Value, value reflect.Value) error {
	if !field.IsValid() {
		return fmt.Errorf("field is not valid")
	}

	if !field.CanSet() {
		return fmt.Errorf("field cannot be set")
	}

	if !value.IsValid() {
		return fmt.Errorf("value is not valid")
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic recovered in safeSetFieldValue: %v\n", r)
		}
	}()

	// Если типы совпадают
	if value.Type().AssignableTo(field.Type()) {
		field.Set(value)
		return nil
	}

	// Обработка указателей
	if field.Kind() == reflect.Ptr {
		elemType := field.Type().Elem()

		// Создаем новый указатель
		newPtr := reflect.New(elemType)

		// Рекурсивно устанавливаем значение для элемента
		if err := safeSetFieldValue(newPtr.Elem(), value); err != nil {
			return err
		}

		field.Set(newPtr)
		return nil
	}

	// Получаем фактическое значение
	var rawValue interface{}
	if value.Kind() == reflect.Interface {
		rawValue = value.Interface()
	} else {
		rawValue = value.Interface()
	}

	// Конвертация в зависимости от типа поля
	switch field.Kind() {
	case reflect.String:
		field.SetString(fmt.Sprint(rawValue))

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch v := rawValue.(type) {
		case int64:
			field.SetInt(v)
		case int32:
			field.SetInt(int64(v))
		case int16:
			field.SetInt(int64(v))
		case int8:
			field.SetInt(int64(v))
		case int:
			field.SetInt(int64(v))
		case uint64:
			// Проверка на переполнение
			if v > 1<<63-1 {
				return fmt.Errorf("uint64 value %d overflows int64", v)
			}
			field.SetInt(int64(v))
		case uint32:
			field.SetInt(int64(v))
		case uint16:
			field.SetInt(int64(v))
		case uint8:
			field.SetInt(int64(v))
		case uint:
			field.SetInt(int64(v))
		case float64:
			// Проверка на потерю точности
			if v < -1<<63 || v >= 1<<63 {
				return fmt.Errorf("float64 value %f overflows int64", v)
			}
			field.SetInt(int64(v))
		case float32:
			field.SetInt(int64(v))
		case []byte:
			// Для []byte конвертируем в строку и парсим
			strVal := strings.TrimSpace(string(v))
			if strVal == "" {
				field.SetInt(0)
				return nil
			}
			intVal, err := strconv.ParseInt(strVal, 10, 64)
			if err != nil {
				return fmt.Errorf("cannot parse []byte '%s' to int64: %w", strVal, err)
			}
			field.SetInt(intVal)
		case string:
			// Убираем пробелы и пробуем распарсить
			strVal := strings.TrimSpace(v)
			if strVal == "" {
				field.SetInt(0)
				return nil
			}
			intVal, err := strconv.ParseInt(strVal, 10, 64)
			if err != nil {
				return fmt.Errorf("cannot parse string '%s' to int64: %w", strVal, err)
			}
			field.SetInt(intVal)
		case bool:
			if v {
				field.SetInt(1)
			} else {
				field.SetInt(0)
			}
		case nil:
			field.SetInt(0)
		default:
			return fmt.Errorf("cannot convert %T to int64", rawValue)
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch v := rawValue.(type) {
		case uint64:
			field.SetUint(v)
		case uint32:
			field.SetUint(uint64(v))
		case uint16:
			field.SetUint(uint64(v))
		case uint8:
			field.SetUint(uint64(v))
		case uint:
			field.SetUint(uint64(v))
		case int64:
			if v < 0 {
				return fmt.Errorf("negative int64 value %d cannot be converted to uint", v)
			}
			field.SetUint(uint64(v))
		case int32:
			if v < 0 {
				return fmt.Errorf("negative int32 value %d cannot be converted to uint", v)
			}
			field.SetUint(uint64(v))
		case int16:
			if v < 0 {
				return fmt.Errorf("negative int16 value %d cannot be converted to uint", v)
			}
			field.SetUint(uint64(v))
		case int8:
			if v < 0 {
				return fmt.Errorf("negative int8 value %d cannot be converted to uint", v)
			}
			field.SetUint(uint64(v))
		case int:
			if v < 0 {
				return fmt.Errorf("negative int value %d cannot be converted to uint", v)
			}
			field.SetUint(uint64(v))
		case []byte:
			strVal := strings.TrimSpace(string(v))
			if strVal == "" {
				field.SetUint(0)
				return nil
			}
			uintVal, err := strconv.ParseUint(strVal, 10, 64)
			if err != nil {
				return fmt.Errorf("cannot parse []byte '%s' to uint64: %w", strVal, err)
			}
			field.SetUint(uintVal)
		case string:
			strVal := strings.TrimSpace(v)
			if strVal == "" {
				field.SetUint(0)
				return nil
			}
			uintVal, err := strconv.ParseUint(strVal, 10, 64)
			if err != nil {
				return fmt.Errorf("cannot parse string '%s' to uint64: %w", strVal, err)
			}
			field.SetUint(uintVal)
		case nil:
			field.SetUint(0)
		default:
			return fmt.Errorf("cannot convert %T to uint64", rawValue)
		}

	case reflect.Bool:
		switch v := rawValue.(type) {
		case bool:
			field.SetBool(v)
		case int64:
			field.SetBool(v != 0)
		case int32:
			field.SetBool(v != 0)
		case int16:
			field.SetBool(v != 0)
		case int8:
			field.SetBool(v != 0)
		case int:
			field.SetBool(v != 0)
		case uint64:
			field.SetBool(v != 0)
		case uint32:
			field.SetBool(v != 0)
		case uint16:
			field.SetBool(v != 0)
		case uint8:
			field.SetBool(v != 0)
		case uint:
			field.SetBool(v != 0)
		case []byte:
			strVal := strings.ToLower(strings.TrimSpace(string(v)))
			field.SetBool(strVal == "true" || strVal == "1" || strVal == "t" ||
				strVal == "yes" || strVal == "y" || strVal == "on")
		case string:
			strVal := strings.ToLower(strings.TrimSpace(v))
			field.SetBool(strVal == "true" || strVal == "1" || strVal == "t" ||
				strVal == "yes" || strVal == "y" || strVal == "on")
		case nil:
			field.SetBool(false)
		default:
			return fmt.Errorf("cannot convert %T to bool", rawValue)
		}

	default:
		// Для других типов пробуем просто установить
		if value.Type().ConvertibleTo(field.Type()) {
			field.Set(value.Convert(field.Type()))
		} else {
			return fmt.Errorf("unsupported field type: %v", field.Kind())
		}
	}

	return nil
}

// Старая функция для обратной совместимости
func setFieldValue(field reflect.Value, value reflect.Value) {
	if err := safeSetFieldValue(field, value); err != nil {
		// Тихий сбой для обратной совместимости
	}
}

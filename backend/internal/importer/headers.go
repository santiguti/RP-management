package importer

import (
	"strings"

	"github.com/xuri/excelize/v2"
)

func headerMap(row []string) map[string]int {
	out := make(map[string]int, len(row))
	for i, h := range row {
		k := strings.ToLower(strings.TrimSpace(h))
		if k == "" {
			continue
		}
		out[k] = i
	}
	return out
}

func cellAt(row []string, hm map[string]int, key string) string {
	idx, ok := hm[key]
	if !ok || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func isBlank(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func strPtrOrNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func firstSheetRows(f *excelize.File) ([][]string, error) {
	name := f.GetSheetName(0)
	if name == "" {
		return nil, nil
	}
	return f.GetRows(name)
}

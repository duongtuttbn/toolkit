package toolkit

import "encoding/json"

func ConvertType[T any](data any) (T, error) {
	var result T
	bytes, err := json.Marshal(data)
	if err != nil {
		return result, err
	}

	err = json.Unmarshal(bytes, &result)
	return result, err
}

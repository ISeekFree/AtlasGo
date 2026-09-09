package common

type Response[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data,omitempty"`
}

func Success[T any](data T) Response[T] {
	return Response[T]{Code: 0, Msg: "success", Data: data}
}

func Failure(code int, msg string) Response[any] {
	return Response[any]{Code: code, Msg: msg}
}

type Paged[T any] struct {
	Total int64 `json:"total"`
	Items []T   `json:"items"`
}

func PageOf[T any](total int64, items []T) Paged[T] {
	return Paged[T]{Total: total, Items: items}
}

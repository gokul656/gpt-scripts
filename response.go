package main

type Response[T any] struct {
	out T
	err error
}

type OpenApiResponse struct {
	Choices []Choice      `json:"choices"`
	Error   *OpenApiError `json:"error"`
}

type OpenApiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   any    `json:"param"`
	Code    string `json:"code"`
}

type Choice struct {
	Index   int     `json:"index"`
	Message Message `json:"message"`
}

type Message struct {
	FuncCall FunctionCall `json:"function_call"`
}

type FunctionCall struct {
	Arguments string `json:"arguments"`
}

type Output struct {
	Questions []Question
}

type Messages struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Function struct {
	Model    string        `json:"model"`
	Messages []interface{} `json:"messages"`
}

type Question struct {
	Title      string `json:"title"`
	TopicName  string `json:"topicName"`
	AnswerType string `json:"answerType"`
	Mmr        string `json:"mmr"`
	Question   string `json:"question"`
	Answer     string `json:"answer"`
	Values     string `json:"values"`
	Hints      string `json:"hints"`
}

type GeneratedQuestion struct {
	Question      string `json:"question"`
	CorrectAnswer string `json:"correctAnswer"`
	Hints         string `json:"hints"`
}

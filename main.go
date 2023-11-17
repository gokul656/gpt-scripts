package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	token        = flag.String("t", "", "Open-AI auth token")
	questionList = []Question{}
)

const (
	startRange = 1
	endRange   = int(startRange + 1)
	url        = "https://api.openai.com/v1/chat/completions"
	model      = "gpt-4-0613"
	outfile    = "output.json"
	infile     = "input.json"
)

func init() {
	flag.Parse()

	if *token == "" {
		panic("Invalid auth token!")
	}
}

func main() {
	start := time.Now()
	defer func() {
		fmt.Println("Time took:", time.Since(start))
		if r := recover(); r != nil {
			fmt.Println("Recovering...")
		}
		saveToFile(outfile)
	}()

	filteredQuestions, err := getSampleQuestions()
	if err != nil {
		panic(err)
	}

	fmt.Println("Total questions:", len(filteredQuestions))

	responseChannel := make(chan *Response[*Question], 1024)
	createOutfile(outfile)

	wg := &sync.WaitGroup{}
	for i := startRange; i < endRange; i++ {
		wg.Add(1)
		question := filteredQuestions[i]
		go createRequest(i, &question, responseChannel, wg)
	}

	wg.Wait()
	close(responseChannel)

	for response := range responseChannel {
		if response.err != nil {
			fmt.Println(response.err)
		} else {
			questionList = append(questionList, *response.out)
		}
	}
}

func getSampleQuestions() ([]Question, error) {
	inputFiles, err := readFile(infile)
	if err != nil {
		return nil, err
	}

	filteredQuestions := []Question{}
	for _, ques := range inputFiles {
		if ques.AnswerType == "Open text" {
			filteredQuestions = append(filteredQuestions, ques)
		}
	}

	return filteredQuestions, nil
}

func createOutfile(outfile string) error {
	// Open a file for writing (create if not exists, truncate if exists)
	_, err := os.Open(outfile)
	if os.IsNotExist(err) {
		fmt.Println("File does not exist. Creating", outfile)
		file, err := os.Create(outfile)
		if err != nil {
			fmt.Println("Error creating file:", err)
			return err
		}
		defer file.Close()
	}

	return nil
}

func readFile(infile string) ([]Question, error) {
	questions := []Question{}
	buffer, err := os.ReadFile(infile)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(buffer, &questions)
	if err != nil {
		return nil, err
	}

	return questions, nil
}

func createRequest(count int, question *Question, output chan *Response[*Question], wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovering go routine...")
		}
	}()

	fmt.Println("Fetching for question #", count)

	message := question.Question
	messages := make(map[string]string)
	messages["content"] = fmt.Sprintf("%s Read the JSON string and generate different questions as the example question provided. Return the output in latex format", message)
	messages["role"] = "system"

	// function contains too make dynamic fields so don't create struct
	function := make(map[string]interface{}, 1024)
	function["model"] = model
	function["messages"] = []interface{}{messages}

	file, _ := os.ReadFile("function.json")
	err := json.Unmarshal(file, &function)
	if err != nil {
		output <- buildError("unable to unmarhsal", err)
		return
	}

	var byteBuffer bytes.Buffer
	err = json.NewEncoder(&byteBuffer).Encode(function)
	if err != nil {
		output <- buildError("unable to encode", err)
		return
	}

	authToken := fmt.Sprintf("Bearer %s", *token)
	request, err := http.NewRequest(http.MethodPost, url, &byteBuffer)
	if err != nil {
		output <- buildError("unable to create request", err)
		return
	}

	// setting auth headers
	header := request.Header
	header.Add("Content-Type", "application/json")
	header.Add("Authorization", authToken)

	// intiating request
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		output <- buildError("http error", err)
		return
	}

	defer response.Body.Close()

	responseBuffer, _ := io.ReadAll(response.Body)

	structResponse := &OpenApiResponse{}
	err = json.Unmarshal(responseBuffer, structResponse)
	if err != nil {
		output <- buildError("unable to unmarhsal", err)
		return
	} else if structResponse.Error != nil {
		output <- buildError("api error", errors.New(structResponse.Error.Message))
		return
	}

	if len(structResponse.Choices) > 0 {
		questionBuffer := []byte(structResponse.Choices[0].Message.FuncCall.Arguments)
		generatedQuestion := &GeneratedQuestion{}
		err = json.Unmarshal(questionBuffer, generatedQuestion)
		if err != nil {
			output <- buildError("unable to unmarhsal", err)
			return
		}

		question.Answer = generatedQuestion.CorrectAnswer
		question.Question = generatedQuestion.Question
		question.Hints = generatedQuestion.Hints

		output <- &Response[*Question]{
			out: question,
			err: nil,
		}

		return
	}

	output <- buildError("choices is empty", errors.New("invalid response"))
}

func buildError(prefix string, err error) *Response[*Question] {
	return &Response[*Question]{
		out: nil,
		err: err,
	}
}

func saveToFile(outfile string) {
	oldQuestions, err := readFile(outfile)
	if err != nil {
		oldQuestions = []Question{}
	}

	oldQuestions = append(oldQuestions, questionList...)

	byteBuffer, _ := json.Marshal(oldQuestions)
	export(byteBuffer, outfile)
}

func export(byteBuffer []byte, outfile string) error {
	file, err := os.OpenFile(outfile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return err
	}
	defer file.Close()

	file.Write(byteBuffer)
	return nil
}

type Response[T any] struct {
	out T
	err error
}

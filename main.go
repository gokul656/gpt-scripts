package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
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
	startRange = 10
	endRange   = int(startRange + 5)
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
		saveToOutfile(outfile)
	}()

	inputFiles, err := readInputFile(infile)
	if err != nil {
		panic(err)
	}

	filteredQuestions := []Question{}
	for _, ques := range inputFiles {
		if ques.AnswerType == "Open text" {
			filteredQuestions = append(filteredQuestions, ques)
		}
	}

	fmt.Println("Total questions:", len(filteredQuestions))

	outchan := make(chan *Question, 10*1024)
	createOutfile(outfile)

	wg := &sync.WaitGroup{}
	for i := startRange; i < endRange; i++ {
		question := filteredQuestions[i]
		wg.Add(1)
		go createRequest(i, question, outchan, wg)
	}

	wg.Wait()
	close(outchan)

	for newQuestion := range outchan {
		questionList = append(questionList, *newQuestion)
	}
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

func readInputFile(infile string) ([]Question, error) {
	questions := []Question{}
	buffer, err := os.ReadFile(infile)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	err = json.Unmarshal(buffer, &questions)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return questions, nil
}

func createRequest(count int, question Question, outchan chan *Question, wg *sync.WaitGroup) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovering go routine...")
		}
	}()
	defer wg.Done()

	fmt.Println("Fetching for question #", count)
	message := ""

	messages := make(map[string]string)
	messages["content"] = fmt.Sprintf("%s Read the JSON string and generate different questions as the example question provided. Return the output in latex format", message)
	messages["role"] = "system"

	function := make(map[string]interface{}, 1024)
	function["model"] = model
	function["messages"] = []interface{}{messages}

	file, _ := os.ReadFile("function.json")
	err := json.Unmarshal(file, &function)
	if err != nil {
		fmt.Println(err.Error())
	}

	var byteBuffer bytes.Buffer
	err = json.NewEncoder(&byteBuffer).Encode(function)
	if err != nil {
		fmt.Println("[ERROR]", err)
	}

	authToken := fmt.Sprintf("Bearer %s", token)
	request, err := http.NewRequest(http.MethodPost, url, &byteBuffer)
	if err != nil {
		log.Println("HTTP ERROR", err)
	}

	// setting auth headers
	header := request.Header
	header.Add("Content-Type", "application/json")
	header.Add("Authorization", authToken)

	// intiating request
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}

	defer response.Body.Close()

	responseBuffer, _ := io.ReadAll(response.Body)

	structResponse := &OpenApiResponse{}
	_ = json.Unmarshal(responseBuffer, structResponse)

	if len(structResponse.Choices) > 0 {
		questionBuffer := []byte(structResponse.Choices[0].Message.FuncCall.Arguments)
		generatedQuestion := &GeneratedQuestion{}
		err = json.Unmarshal(questionBuffer, generatedQuestion)
		if err != nil {
			fmt.Println("ERROR parsing", err)
			return
		}

		question.Answer = generatedQuestion.CorrectAnswer
		question.Question = generatedQuestion.Question
		question.Hints = generatedQuestion.Hints

		outchan <- &question
	}
}

func saveToOutfile(outfile string) {
	oldQuestions, err := readInputFile(outfile)
	if err != nil {
		fmt.Println("Unable to read old outfile")
		oldQuestions = []Question{}
	}

	oldQuestions = append(oldQuestions, questionList...)

	byteBuffer, _ := json.Marshal(oldQuestions)
	file, err := os.OpenFile(outfile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	file.Write(byteBuffer)
}

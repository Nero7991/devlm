package analyzer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/Nero7991/devlm/internal/core/nlp"
)

type RequirementAnalyzer struct {
	filePath string
	nlpUtils *nlp.NLPUtils
	config   *Config
}

type Config struct {
	MaxLineLength int             `json:"max_line_length"`
	Weights       priorityWeights `json:"weights"`
}

type priorityWeights struct {
	Priority float64 `json:"priority"`
	Impact   float64 `json:"impact"`
	Effort   float64 `json:"effort"`
	Urgency  float64 `json:"urgency"`
}

func NewRequirementAnalyzer(filePath string, configPath string) (*RequirementAnalyzer, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	config, err := loadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("error loading config: %w", err)
	}

	if err := validateFileContent(filePath, config.MaxLineLength); err != nil {
		return nil, fmt.Errorf("invalid file content: %w", err)
	}

	return &RequirementAnalyzer{
		filePath: filePath,
		nlpUtils: nlp.NewNLPUtils(),
		config:   config,
	}, nil
}

func loadConfig(configPath string) (*Config, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func validateFileContent(filePath string, maxLineLength int) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
		if len(scanner.Text()) > maxLineLength {
			return fmt.Errorf("line %d exceeds maximum length of %d characters", lineCount, maxLineLength)
		}
	}

	if lineCount == 0 {
		return fmt.Errorf("file is empty")
	}

	return scanner.Err()
}

func (ra *RequirementAnalyzer) AnalyzeRequirements() ([]string, error) {
	file, err := os.Open(ra.filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	var requirements []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			requirements = append(requirements, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return requirements, nil
}

func (ra *RequirementAnalyzer) ExtractKeywords(requirements []string) []string {
	keywords := make(map[string]int)
	for _, req := range requirements {
		words := ra.nlpUtils.Tokenize(req)
		for _, word := range words {
			word = strings.ToLower(word)
			if len(word) > 3 && ra.nlpUtils.IsRelevantKeyword(word) {
				keywords[word]++
			}
		}
	}

	var result []string
	for keyword, count := range keywords {
		if count > 1 {
			result = append(result, keyword)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return keywords[result[i]] > keywords[result[j]]
	})

	return result[:min(len(result), 20)]
}

func (ra *RequirementAnalyzer) CategorizeRequirements(requirements []string) map[string][]string {
	categories := make(map[string][]string)
	for _, req := range requirements {
		category := ra.nlpUtils.ClassifyRequirement(req)
		categories[category] = append(categories[category], req)
	}

	if functionalReqs := ra.nlpUtils.ExtractFunctionalRequirements(requirements); len(functionalReqs) > 0 {
		categories["Functional"] = functionalReqs
	}
	if nonFunctionalReqs := ra.nlpUtils.ExtractNonFunctionalRequirements(requirements); len(nonFunctionalReqs) > 0 {
		categories["Non-Functional"] = nonFunctionalReqs
	}

	categories["Security"] = ra.nlpUtils.ExtractSecurityRequirements(requirements)
	categories["Performance"] = ra.nlpUtils.ExtractPerformanceRequirements(requirements)
	categories["Usability"] = ra.nlpUtils.ExtractUsabilityRequirements(requirements)
	categories["Maintainability"] = ra.nlpUtils.ExtractMaintainabilityRequirements(requirements)
	categories["Scalability"] = ra.nlpUtils.ExtractScalabilityRequirements(requirements)

	return categories
}

func (ra *RequirementAnalyzer) PrioritizeRequirements(requirements []string) []string {
	type reqPriority struct {
		req      string
		priority float64
	}

	prioritized := make([]reqPriority, len(requirements))
	for i, req := range requirements {
		priority := ra.nlpUtils.AssessPriority(req)
		impact := ra.nlpUtils.AssessImpact(req)
		effort := ra.nlpUtils.AssessEffort(req)
		urgency := ra.nlpUtils.AssessUrgency(req)

		weights := ra.config.Weights
		overallPriority := float64(priority)*weights.Priority +
			float64(impact)*weights.Impact -
			float64(effort)*weights.Effort +
			float64(urgency)*weights.Urgency

		prioritized[i] = reqPriority{req: req, priority: overallPriority}
	}

	sort.Slice(prioritized, func(i, j int) bool {
		return prioritized[i].priority > prioritized[j].priority
	})

	result := make([]string, len(prioritized))
	for i, rp := range prioritized {
		result[i] = rp.req
	}

	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
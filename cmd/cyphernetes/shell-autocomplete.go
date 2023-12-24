package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/avitaltamir/cyphernetes/pkg/parser"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var resourceTreeStructureCache = make(map[string][]string)

type CyphernetesCompleter struct {
	// You can add fields if needed, for example, a reference to your GRV cache
}

func (c *CyphernetesCompleter) Do(line []rune, pos int) ([][]rune, int) {
	var suggestions [][]rune

	// Extract the last word from the line up to the cursor position.
	// This assumes words are separated by spaces.
	lineStr := string(line[:pos])
	if len(lineStr) < 1 {
		// If the line is empty, return no suggestions
		return suggestions, 0
	}

	words := strings.Fields(lineStr)
	lastWord := words[len(words)-1]
	prefix := strings.ToLower(lastWord)

	if len(words) > 0 {

		// Check if the last word starts with a '(', followed by a word, followed by a colon and we're now directly after the colon
		// make the check with regex:
		resourceKindIdentifierRegex := regexp.MustCompile(`\(\w+:\w+$`)
		if resourceKindIdentifierRegex.MatchString(lastWord) {
			identifier := strings.SplitAfter(lastWord, ":")[1]
			resourceKinds := getResourceKinds(identifier)
			for _, kind := range resourceKinds {
				if strings.HasPrefix(kind, identifier) {
					// Offer each kind as a suggestion
					// everything from the last word after the colon:
					suggestion := string(kind)[len(identifier):]
					suggestions = append(suggestions, []rune(suggestion))
				}
			}
			// Set the length to 0 since we want to append the entire resource kind
		} else if isJSONPathContext(lineStr, pos, lastWord) {
			identifier := strings.Split(lastWord, ".")[0]
			kind := getKindForIdentifier(lineStr, identifier)
			treeStructure, err := fetchResourceTreeStructureForKind(kind)
			// matched word is last word with identifier replaced by '$':
			matchedWord := strings.Replace(lastWord, identifier, "$", 1)
			treeStructure = append(treeStructure, metadataJsonPaths...)
			if err == nil {
				for _, node := range treeStructure {
					if strings.HasPrefix(node, matchedWord) {
						// Offer each kind as a suggestion
						// everything from the last word after the colon:
						suggestion := strings.Replace(string(node), "$", identifier, 1)[len(lastWord):]
						suggestions = append(suggestions, []rune(suggestion))
					}
				}
			} else {
				fmt.Println("Error fetching resource tree structure: ", err)
			}
		} else {
			// Handle other autocompletion cases (like keywords)

			// Keywords
			keywords := []string{"match", "return", "set", "delete"} //, "create", "as", "where"}

			for _, k := range keywords {
				if strings.HasPrefix(k, prefix) {
					// Append only the part of the keyword that comes after the prefix
					suggestion := k[len(prefix):]
					suggestions = append(suggestions, []rune(suggestion))
				}
			}
		}
	}

	// Check if the last word is a resource kind, which means we're after a '(', followed by a word, followed by a colon and we're now directly after the colon
	// make the check:

	// The length returned should be the length of the last word.
	return suggestions, len(lastWord)
}

func fetchResourceTreeStructureForKind(kind string) ([]string, error) {
	// first get the full name of the kind from the gvr cache
	gvr, err := parser.FindGVR(executor.Clientset, kind)
	if err == nil {
		resourceNormalizedName := strings.ToLower(gvr.Resource)
		// then get the tree structure from the cache, otherwise fetch it from the api
		if treeStructure, ok := resourceTreeStructureCache[resourceNormalizedName]; ok {
			return treeStructure, nil
		} else {
			treeStructure, err := fetchResourceAPIDefinition(gvr)
			if err != nil {
				return []string{}, err
			}

			resourceTreeStructureCache[resourceNormalizedName] = treeStructure
			return treeStructure, nil
		}
	}
	return []string{}, nil
}

func fetchResourceAPIDefinition(gvr schema.GroupVersionResource) ([]string, error) {

	results := []string{}
	// TODO: fetch the resource definition from the api

	// get a count of the number of paths and schemas.
	if resourceSpecs[gvr.Resource] == nil {
		return results, nil
	}
	results = resourceSpecs[gvr.Resource]
	return results, nil
}

func getKindForIdentifier(line string, identifier string) string {
	result := ""
	// Regular expression to find the position of words after '(' and before ':'
	regex := regexp.MustCompile(`\((\w+):(\w+)`)
	matches := regex.FindAllStringSubmatch(line, -1)
	for _, match := range matches {
		if match[1] == identifier {
			// return the last capture group
			return match[2]
		}
	}

	return result
}

func getResourceKinds(identifier string) []string {
	// iterate over the gvr cache and return all resource kinds that match the identifier
	var kinds []string
	for _, gvr := range parser.GvrCache {
		if strings.HasPrefix(gvr.GroupResource().Resource, identifier) {
			kinds = append(kinds, gvr.Resource)
		}
	}
	return kinds
}

func isJSONPathContext(line string, pos int, lastWord string) bool {
	// Regular expression to find the position of "RETURN" and any JSONPaths after it
	regex := regexp.MustCompile(`(?i)(return|set)(\s+.*)(,|$)`)
	matches := regex.FindAllStringSubmatchIndex(line, -1)
	for _, match := range matches {
		wordRegex := regexp.MustCompile(`\w+\.|\$\.\w+\.|\$\.\w+\.(\w)|\w+\.(\w+)`)
		if wordRegex.MatchString(lastWord) {
			if pos > match[2] && ((pos == len(line) && len(line) > 20) || pos < match[3]) {
				return true
			}
		}
	}
	return false
}

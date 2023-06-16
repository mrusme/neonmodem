package common

import (
	"github.com/manifoldco/promptui"
)

func YesNo() (bool, error) {
	prompt := promptui.Select{
		Label: "Select[Yes/No]",
		Items: []string{"Yes", "No"},
	}

	_, result, err := prompt.Run()
	if err != nil {
		return false, err
	}

	return result == "Yes", nil

}

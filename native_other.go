//go:build !darwin

package main

func hideDockIcon() {}

func setAppAppearance(theme string) {} // Windows uses the runtime theme calls; Linux follows GTK

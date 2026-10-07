// Package tools only pins the development and CI tools declared with tool
// directives in go.mod. It has no code of its own; it exists so that
// govulncheck (run by osv-scanner's call analysis) finds a package to load.
package tools

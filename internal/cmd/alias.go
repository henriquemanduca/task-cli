package cmd

// FlatAliases maps legacy flat command names and shortcuts to their hierarchical routes in Root.
var FlatAliases = map[string][]string{
	// Projects
	"np":             {"project", "add"},
	"project-add":    {"project", "add"},
	"lp":             {"project", "list"},
	"projects":       {"project", "list"},
	"project-update": {"project", "update"},

	// Tasks
	"add":             {"task", "add"},
	"list":            {"task", "list"},
	"ls":              {"task", "list"},
	"view":            {"task", "view"},
	"show":            {"task", "view"},
	"get":             {"task", "view"},
	"update":          {"task", "update"},
	"mod":             {"task", "update"},
	"edit":            {"task", "edit"},
	"delete":          {"task", "delete"},
	"del":             {"task", "delete"},
	"rm":              {"task", "delete"},
	"remove":          {"task", "delete"},
	"status":          {"task", "status"},
	"set-status":      {"task", "status"},
	"task-status":     {"task", "status"},
	"prio":            {"task", "prio"},
	"priority":        {"task", "prio"},
	"set-prio":        {"task", "prio"},
	"set-priority":    {"task", "prio"},
	"set-pending":     {"task", "set-pending"},
	"pending":         {"task", "set-pending"},
	"set-in-progress": {"task", "set-in-progress"},
	"start":           {"task", "set-in-progress"},
	"set-done":        {"task", "set-done"},
	"done":            {"task", "set-done"},
	"set-abort":       {"task", "set-abort"},
	"abort":           {"task", "set-abort"},
	"set-aborted":     {"task", "set-abort"},
	"aborted":         {"task", "set-abort"},

	// Documents
	"attach":     {"task", "doc", "add"},
	"doc-add":    {"task", "doc", "add"},
	"docs":       {"task", "doc", "list"},
	"doc-list":   {"task", "doc", "list"},
	"doc-del":    {"task", "doc", "remove"},
	"doc-rm":     {"task", "doc", "remove"},
	"doc-remove": {"task", "doc", "remove"},
	"doc-open":   {"doc", "open"},
	"doc-prune":  {"doc", "prune"},

	// I/O
	"export": {"export"},
	"exp":    {"export"},
	"import": {"import"},
	"imp":    {"import"},
}

// ResolveFlatAlias checks if the given alias exists in FlatAliases.
// If found, it returns a clone of the target hierarchical path and true;
// otherwise, it returns nil and false.
func ResolveFlatAlias(alias string) ([]string, bool) {
	target, ok := FlatAliases[alias]
	if !ok {
		return nil, false
	}
	res := make([]string, len(target))
	copy(res, target)
	return res, true
}

// ExpandAliases checks if the first argument in args is a registered flat alias.
// If it is, it replaces args[0] with the corresponding hierarchical command path.
// Otherwise, it returns args unchanged.
func ExpandAliases(args []string) []string {
	if len(args) == 0 {
		return args
	}
	targetPath, ok := ResolveFlatAlias(args[0])
	if !ok {
		return args
	}
	expanded := make([]string, 0, len(targetPath)+len(args)-1)
	expanded = append(expanded, targetPath...)
	expanded = append(expanded, args[1:]...)
	return expanded
}

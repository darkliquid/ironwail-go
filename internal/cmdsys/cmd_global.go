package cmdsys

func AddCommand(name string, fn CommandFunc, desc string) { globalCmd.AddCommand(name, fn, desc) }
func AddClientCommand(name string, fn CommandFunc, desc string) {
	globalCmd.AddClientCommand(name, fn, desc)
}
func AddServerCommand(name string, fn CommandFunc, desc string) {
	globalCmd.AddServerCommand(name, fn, desc)
}
func RemoveCommand(name string)                           { globalCmd.RemoveCommand(name) }
func AddAlias(name, command string)                       { globalCmd.AddAlias(name, command) }
func RemoveAlias(name string) bool                        { return globalCmd.RemoveAlias(name) }
func UnaliasAll()                                         { globalCmd.UnaliasAll() }
func Alias(name string) (string, bool)                    { return globalCmd.Alias(name) }
func Aliases() map[string]string                          { return globalCmd.Aliases() }
func AddText(text string)                                 { globalCmd.AddText(text) }
func AddTextWithSource(text string, source CommandSource) { globalCmd.AddTextWithSource(text, source) }
func InsertText(text string)                              { globalCmd.InsertText(text) }
func InsertTextWithSource(text string, source CommandSource) {
	globalCmd.InsertTextWithSource(text, source)
}
func Execute()                               { globalCmd.Execute() }
func ExecuteWithSource(source CommandSource) { globalCmd.ExecuteWithSource(source) }
func ExecuteText(text string)                { globalCmd.ExecuteText(text) }
func ExecuteTextWithSource(text string, source CommandSource) {
	globalCmd.ExecuteTextWithSource(text, source)
}
func SetSource(source CommandSource)          { globalCmd.SetSource(source) }
func Source() CommandSource                   { return globalCmd.Source() }
func Exists(name string) bool                 { return globalCmd.Exists(name) }
func Complete(partial string) []string        { return globalCmd.Complete(partial) }
func CompleteAliases(partial string) []string { return globalCmd.CompleteAliases(partial) }
func SetCommandCompletion(name string, completion func(args []string, partial string) []string) {
	globalCmd.SetCommandCompletion(name, completion)
}
func CompleteCommandArgs(cmdName string, args []string, partial string) []string {
	return globalCmd.CompleteCommandArgs(cmdName, args, partial)
}
func SetForwardFunc(fn func(line string)) { globalCmd.ForwardFunc = fn }

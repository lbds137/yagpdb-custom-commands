{{ $key := "" }}
{{ $userID := "" }}
{{ if .ExecData.Key }}
    {{ $key = .ExecData.Key }}
    {{ $userID = or .ExecData.UserID .User.ID }}
{{ else if gt (len .CmdArgs) 0 }}
    {{ $key = index .CmdArgs 0 }}
    {{ $userID = .User.ID }}
{{ end }}
 
{{ $title := "" }}
{{ $result := "" }}
{{ if $key }}
    {{ $title = $key }}
    {{ $result = or (dbGet .User.ID $key).Value "(no result found)" }}
{{ else }}
    {{ $title = "Missing Argument" }}
    {{ $result = "⚠️ You did not provide a key to look up!" }}
{{ end }}
{{ execCC 3 nil 0 (sdict "Title" $title "Description" $result) }}
{{ deleteTrigger 0 }}
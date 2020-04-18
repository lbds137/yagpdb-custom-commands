{{ $key := "" }}
{{ if gt (len .CmdArgs) 0 }}
    {{ $key = index .CmdArgs 0 }}
{{ end }}

{{ $title := "" }}
{{ $result := "" }}
{{ if $key }}
    {{ $title = $key }}
    {{ $result = (dbGet .User.ID $key).Value }}
{{ else }}
    {{ $title = "Missing Argument" }}
    {{ $result = "⚠️ You did not provide a key to look up!" }}
{{ end }}
{{ execCC 3 nil 0 (sdict "Key" $title "Value" $result) }}
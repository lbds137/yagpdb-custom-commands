{{ $trigger := .Message.Content }}
{{ if not (reFind "y\\^agree" $trigger) }}
    {{ $result := "⚠️ You must enter the correct command for agreeing to the rules: `y^agree`" }}
    {{ execCC 3 nil 0 (sdict "Title" "Invalid Agreement Command" "Description" $result "DeleteResponse" true "DeleteDelay" 5) }}
    {{ deleteTrigger 0 }}
    {{ deleteResponse 5 }}
{{ end }}

{{ $trigger := .Message.Content }}
{{ if not (reFind "y\\^ticket open .+" $trigger) }}
    {{ $result := joinStr "" 
        "⚠️ You must enter a proper ticket open command: `y^ticket open [reason]`\n\n"
        " Replace `[reason]` with a brief but descriptive reason for opening the ticket, e.g. `concern about another member`." 
    }}
    {{ execCC 3 nil 0 (sdict "Title" "Invalid Ticket Command" "Description" $result "DeleteResponse" true "DeleteDelay" 10) }}
    {{ deleteTrigger 0 }}
    {{ deleteResponse 5 }}
{{ end }}

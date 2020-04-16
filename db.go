{{ $operation := "" }}
{{ $key := "" }}
{{ $value := "" }}
{{ $member := "" }}

{{ if .ExecData }}
    {{ $operation = .ExecData.Operation }}
    {{ $key = .ExecData.Key }}
    {{ $value = .ExecData.Value }}
    {{ $member = getMember .ExecData.UserID }}
{{ else }}
    {{ $args := parseArgs 2 (joinStr "" "Usage: `del|get|set` `key` `value`")
        (carg "string" "operation")
        (carg "string" "key")
        (carg "string" "value (optional)")
    }}
    {{ $operation = $args.Get 0 }}
    {{ $key = $args.Get 1 }}
    {{ $value = and ($args.IsSet 2) ($args.Get 2) }}
    {{ $member = .Member }}
{{ end }}

{{ $isDel := eq $operation "del" }}
{{ $isGet := eq $operation "get" }}
{{ $isSet := eq $operation "set" }}
{{ $operationCheck := or $isDel $isGet $isSet }}
{{ $valueCheck := or $value $isDel $isGet }}

{{ $result := "" }}
{{ $resultText := "" }}
{{ $resultEmoji := "✅" }}
{{ if and $operationCheck $key $valueCheck }}
    {{ $ptAction := "" }}
    {{ if or $isDel $isGet }}
        {{ $result = (dbGet $member.User.ID $key).Value }}
        {{ if $isDel }}
            {{ $ptAction = "deleted" }}
            {{ dbDel $member.User.ID $key }} 
        {{ else if $isGet }}
            {{ $ptAction = "retrieved" }}
        {{ end }}
    {{ else if $isSet }}
        {{ $ptAction = "set" }}
        {{ dbSet $member.User.ID $key $value }} 
    {{ end }}

    {{ if or $result $value }}
        {{ $resultText = joinStr "" "Value for key `" $key "` " $ptAction ": `" (or $result $value) "`\n\n" }}
    {{ else }}
        {{ $result = "(no result found)" }}
        {{ $resultEmoji = "⚠️" }}
        {{ $resultText = joinStr "" "No value found for key: `" $key "`\n\n" }}
    {{ end }}
{{ else }}
    {{ $resultEmoji = "⚠️" }}
    {{ if not $operationCheck }}
        {{ $resultText = joinStr "" "Invalid operation provided: `" $operation "`" }}
    {{ else if not $key }}
        {{ $resultText = "You must provide a key!" }}
    {{ else if not $valueCheck }}
        {{ $resultText = "You must provide a value!" }}
    {{ end }}
{{ end }}

{{ $resultText = joinStr " " $resultEmoji $resultText }}

{{ if and .ExecData $isGet }}
    {{ execCC 3 nil 0 (sdict "Key" $key "Value" (or $result $value)) }}
{{ else }}
    {{ $userFull := $member.User.String }}
    {{ if $member.Nick }}
        {{ $userFull = joinStr "" $member.Nick " (" $member.User.String ")" }}
    {{ end }}
    {{ $userLink := joinStr "" "https://discordapp.com/users/" $member.User.ID }}
    {{ $uAvatar := joinStr "" "https://cdn.discordapp.com/avatars/" $member.User.ID "/" $member.User.Avatar ".gif" }}
    {{ $author := sdict "name" $userFull "url" $userLink "icon_url" $uAvatar }}

    {{ $embed := cembed
        "title" (joinStr "" "Database Operation: `" $operation "`")
        "description" $resultText
        "color" 0xff0000
        "author" $author
    }}

    {{ sendMessage nil $embed }}
{{ end }}

{{ deleteTrigger 5 }}
{{ $operation := "" }}
{{ if ge (len .CmdArgs) 1 }}
    {{ $operation = index .CmdArgs 0 }}
{{ end }}

{{ $key := "" }}
{{ if ge (len .CmdArgs) 2 }}
    {{ $key = index .CmdArgs 1 }}
{{ end }}

{{ $value := "" }}
{{ if ge (len .CmdArgs) 3 }}
    {{ $value = index .CmdArgs 2 }}
{{ end }}

{{ $member := .Member }}

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
        {{ $resultText = joinStr "" "Value for key `" $key "` " $ptAction ": `" (or $result $value) "`" }}
    {{ else }}
        {{ $result = "(no result found)" }}
        {{ $resultEmoji = "⚠️" }}
        {{ $resultText = joinStr "" "No value found for key: `" $key "`" }}
    {{ end }}
{{ else }}
    {{ $resultEmoji = "⚠️" }}
    {{ if not $operationCheck }}
        {{ $resultText = joinStr "" "Invalid operation provided: `" (or $operation "(missing)") "`" }}
    {{ else if not $key }}
        {{ $resultText = "You must provide a key!" }}
    {{ else if not $valueCheck }}
        {{ $resultText = "You must provide a value!" }}
    {{ end }}
{{ end }}

{{ $resultText = joinStr " " $resultEmoji $resultText }}

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
    "color" (toInt (dbGet .Guild.OwnerID "Embed Color").Value)
    "author" $author
}}

{{ sendMessage nil $embed }}

{{ deleteTrigger 5 }}
{{ $args := parseArgs 2 (joinStr "" "Usage: `del|get|set` `key` `value`")
    (carg "string" "operation")
    (carg "string" "key")
    (carg "string" "value (optional)")
}}

{{ $operation := $args.Get 0 }}
{{ $key := $args.Get 1 }}
{{ $value := and ($args.IsSet 2) ($args.Get 2) }}

{{ $isDel := eq $operation "del" }}
{{ $isGet := eq $operation "get" }}
{{ $isSet := eq $operation "set" }}
{{ $operationCheck := or $isDel $isGet $isSet }}
{{ $valueCheck := or $value $isDel $isGet }}

{{ $result := "" }}
{{ $resultText := "" }}
{{ $resultEmoji := "✅" }}
{{ if and $operationCheck $key $valueCheck }}
    {{ if $isDel }}
        {{ dbDel .User.ID $key }} 
    {{ else if $isGet }}
        {{ $result = dbGet .User.ID $key }}
	{{ if $result.Value }} 
        	{{ $resultText = joinStr "" "Value retrieved: `" $result.Value "`\n\n" }}
	{{ else }}
		{{ $resultText = joinStr "" "No value found for key: `" $key "`\n\n" }}
	{{ end }}
    {{ else if $isSet }}
	{{ $resultText = joinStr "" "Value added: `" $value "`\n\n" }}
        {{ dbSet .User.ID $key $value }} 
    {{ end }}
    {{ $resultText = joinStr "" $resultText $resultEmoji " Operation `" $operation "` for key `" $key "` successfully completed!" }}
{{ else }}
    {{ $resultEmoji = "⚠️" }}
    {{ if not $operationCheck }}
        {{ $resultText = joinStr "" $resultText $resultEmoji " Invalid operation provided: `" $operation "`" }}
    {{ else if not $key }}
        {{ $resultText = joinStr "" $resultText $resultEmoji " You must provide a key!" }}
    {{ else if not $valueCheck }}
        {{ $resultText = joinStr "" $resultText $resultEmoji " You must provide a value!" }}
    {{ end }}
{{ end }}

{{ $userFull := .User.String }}
{{ if .Member.Nick }}
    {{ $userFull = joinStr "" .Member.Nick " (" .User.String ")" }}
{{ end }}
{{ $userLink := joinStr "" "https://discordapp.com/users/" .User.ID }}
{{ $uAvatar := joinStr "" "https://cdn.discordapp.com/avatars/" .User.ID "/" .User.Avatar ".gif" }}
{{ $author := sdict "name" $userFull "url" $userLink "icon_url" $uAvatar }}
{{ $embed := cembed
    "title" "Database Operation"
    "description" $resultText
    "color" 0xff0000
    "author" $author
}}

{{ sendMessage nil $embed }}
{{ deleteTrigger 0 }}
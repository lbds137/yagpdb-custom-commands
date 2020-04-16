{{ $args := parseArgs 3 (joinStr "" "Usage: `del|get|set` `key` `value`")
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
{{ $valueCheck := or $value $isGet }}

{{ if and $operationCheck $key $valueCheck }}
    {{ $resultEmoji := "✅" }}
    {{ if $isDel }}
        {{ dbDel .User.ID $key $value }} 
    {{ else if $isGet }}
        {{ dbGet .User.ID $key }} 
    {{ else if $isSet }}
        {{ dbSet .User.ID $key $value }} 
    {{ end }}

    {{ joinStr "" $resultEmoji " Successful `" $operation "` of `" $key ", " $value "` for <@" .User.ID ">!" }} 
{{ else }}
    {{ $resultEmoji := "⚠️" }}
    {{ if not $operationCheck }}
        {{ joinStr "" $resultEmoji " Invalid operation provided: `" $operation "`" }}
    {{ else if not $key }}
        {{ joinStr "" $resultEmoji " You must provide a key!" }}
    {{ else if not $valueCheck }}
        {{ joinStr "" $resultEmoji " You must provide a value!" }}
    {{ end }}
{{ end }}

{{ deleteTrigger 0 }}
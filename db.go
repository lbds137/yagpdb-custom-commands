{{ $args := parseArgs 3 (joinStr "" "Usage: `del|get|set` `key` `value`")
    (carg "string" "operation (`del`, `get`, or `set`)")
    (carg "string" "key")
    (carg "string" "value")
}}

{{ $operation := $args.Get 0 }}
{{ $key := $args.Get 1 }}
{{ $value := $args.Get 2 }}

{{ $isDel := eq $operation "del" }}
{{ $isGet := eq $operation "get" }}
{{ $isSet := eq $operation "set" }}
{{ $operationCheck := or $isDel $isGet $isSet }}
{{ $valueCheck := or $value $isGet }}

{{ if and $operationCheck $key $valueCheck }}
    {{ if $isDel }}
        {{ dbDel .User.ID $key $value }} 
    {{ else if $isGet }}
        {{ dbGet .User.ID $key }} 
    {{ else if $isSet }}
        {{ dbSet .User.ID $key $value }} 
    {{ end }}
{{ else }}
    {{ $errEmoji := "⚠️" }}
    {{ if not $operationCheck }}
        {{ $errEmoji }} Invalid operation provided: {{ joinStr "" "`" $operation "`" }}
    {{ else if not $key }}
        {{ $errEmoji }} You must provide a key!
    {{ else if not $valueCheck }}
        {{ $errEmoji }} You must provide a value!
    {{ end }}
{{ end }}
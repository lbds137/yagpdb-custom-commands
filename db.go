{{ $args := parseArgs 3 (joinStr "" "Usage: `get|set` `key` `value`")
    (carg "string" "operation (`get` or `set`)")
    (carg "string" "key")
    (carg "string" "value")
}}

{{ $operation := $args.Get 0 }}
{{ $key := $args.Get 1 }}
{{ $value := $args.Get 2 }}

{{ $isGet := eq $operation "get" }}
{{ $isSet := eq $operation "set" }}
{{ $operationCheck := or $isGet $isSet }}
{{ $valueCheck := or $value $isGet }}

{{ if and $operationCheck $key $valueCheck }}
    {{ if $isGet }}
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
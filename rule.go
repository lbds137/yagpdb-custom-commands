{{ $min := 1 }}
{{ $max := 10 }}

{{ $args := parseArgs 1 (joinStr "" "Please enter a valid rule number (" $min "-" $max ").")
    (carg "int" "rule number to view")
}}

{{ $n := $args.Get 0 }}

{{ if and (ge $n $min) (le $n $max) }}
    {{ $ccID := 0 }}
    {{ if eq $n 1 }}
        {{ $ccID = 4 }}
    {{ else if eq $n 2 }}
        {{ $ccID = 5 }}
    {{ else if eq $n 3 }}
        {{ $ccID = 6 }}
    {{ else if eq $n 4 }}
        {{ $ccID = 7 }}
    {{ else if eq $n 5 }}
        {{ $ccID = 8 }}
    {{ else if eq $n 6 }}
        {{ $ccID = 9 }}
    {{ else if eq $n 7 }}
        {{ $ccID = 10 }}
    {{ else if eq $n 8 }}
        {{ $ccID = 11 }}
    {{ else if eq $n 9 }}
        {{ $ccID = 12 }}
    {{ else if eq $n 10 }}
        {{ $ccID = 13 }}
    {{ end }}
    
    {{ execCC $ccID nil 0 (sdict "RuleNumber" $n) }}
{{ else }}
    ⚠️ Could not find the requested rule number: {{ joinStr "" "`" $n "`" }}
{{ end }}

{{ deleteTrigger 5 }}
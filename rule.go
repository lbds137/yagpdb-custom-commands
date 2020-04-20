{{ $min := 1 }}
{{ $max := 10 }}
 
{{ $args := parseArgs 1 (joinStr "" "Please enter a valid rule number (" $min "-" $max ").")
    (carg "int" "rule number to view")
}}
 
{{ $n := $args.Get 0 }}
 
{{ if and (ge $n $min) (le $n $max) }}
    {{ $key := joinStr "" "Rule #" $n }}
    {{ $result := (dbGet .Guild.OwnerID $key).Value }}
    {{ execCC 3 nil 0 (sdict "Title" $key "Description" $result) }}
{{ else }}
    ⚠️ Could not find the requested rule number: {{ joinStr "" "`" $n "`" }}
{{ end }}
 
{{ deleteTrigger 5 }}
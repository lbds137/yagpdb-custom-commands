{{ $hexDict := sdict
	"0" 0 "1" 1 "2" 2 "3" 3 "4" 4 "5" 5 "6" 6 "7" 7 "8" 8
	"9" 9 "a" 10 "b" 11 "c" 12 "d" 13 "e" 14 "f" 15
}}

{{ $trigger := lower .Message.Content }}
{{ $tSlice := split (slice $trigger 1) "" }}

{{ $intVal := 0 }}
{{ range $i, $iRevNeg := seq -5 1 }}
	{{ $iRev := mult -1 $iRevNeg }}
	{{ $curValHexStr := index $tSlice $iRev }}
	{{ $curValInt := index $hexDict $curValHexStr }}
	{{ $intVal = add $intVal (mult (pow 16 $i) $curValInt) }}
{{ end }}

{{ $result := (joinStr "" "The `int` value of `" $trigger "` is `" $intVal "`.") }}
{{ execCC 3 nil 0 (sdict "Title" "Color Conversion: `hex` to `int`" "Description" $result "Color" $intVal) }}
{{ deleteTrigger 5}}
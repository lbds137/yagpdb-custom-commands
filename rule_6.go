{{ $rText := "**Do not engage in any magickal workings on other members of the server without consent.** This includes but is not limited to energy work, readings, and spells. **We take this very seriously and will take appropriate corrective action if we receive complaints from our members.**" }}
 
{{ execCC 3 nil 0 (sdict "RuleNumber" .ExecData.RuleNumber "RuleText" $rText) }}
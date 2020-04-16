{{ $rText := "**Do not use offensive slurs that target people’s identities.** This server is sensitive to people’s identities, whatever form they may take, and as such comments that are racist, homophobic, transphobic, etc. will not be tolerated." }}
 
{{ execCC 3 nil 0 (sdict "RuleNumber" .ExecData.RuleNumber "RuleText" $rText) }}
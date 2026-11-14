(declare-const a Bool)
(assert a)
(check-sat-using (using-params sat :scc.tr false))

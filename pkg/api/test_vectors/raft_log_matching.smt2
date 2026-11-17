; Frozen, reduced choosable_future_term_log VC from the Raft slow case.
; Eight finite bit-vector domains preserve the original randomized-test model.
; No Ivy parser, external examples checkout, or GoIvy dependency is required.
; The retained negated goal is choosable_future_term_log.
; Derived from raft_no_assume.ivy; original license follows.
;
; Copyright (c) 2025 James R. Wilcox
;
; Permission is hereby granted, free of charge, 
; to any person obtaining a copy of this software and 
; associated documentation files (the "Software"), 
; to deal in the Software without restriction, 
; including without limitation the rights to use, 
; copy, modify, merge, publish, distribute, sublicense, 
; and/or sell copies of the Software, and to permit 
; persons to whom the Software is furnished to do so, 
; subject to the following conditions:
;
; The above copyright notice and this permission notice 
; shall be included in all copies or substantial 
; portions of the Software.
;
; THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY 
; OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT 
; LIMITED TO THE WARRANTIES OF MERCHANTABILITY, 
; FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. 
; IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS 
; BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, 
; WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, 
; ARISING FROM, OUT OF OR IN CONNECTION WITH THE 
; SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

(declare-fun __term_log ((_ BitVec 3)) (_ BitVec 4))
(declare-fun current_log ((_ BitVec 2)) (_ BitVec 4))
(declare-fun |__fml:n| () (_ BitVec 2))
(declare-fun |__loc:t| () (_ BitVec 3))
(declare-fun term_log ((_ BitVec 3)) (_ BitVec 4))
(declare-fun __term_leader ((_ BitVec 3)) (_ BitVec 2))
(declare-fun term_leader ((_ BitVec 3)) (_ BitVec 2))
(declare-fun __voting_quorum ((_ BitVec 3)) (_ BitVec 4))
(declare-fun |__loc:q| () (_ BitVec 4))
(declare-fun voting_quorum ((_ BitVec 3)) (_ BitVec 4))
(declare-fun none_term () (_ BitVec 4))
(declare-fun none_index () (_ BitVec 4))
(declare-fun commit_term () (_ BitVec 3))
(declare-fun commit_index () (_ BitVec 3))
(declare-fun commit_log () (_ BitVec 4))
(declare-fun value_at ((_ BitVec 4) (_ BitVec 3)) (_ BitVec 2))
(declare-fun commit_value () (_ BitVec 2))
(declare-fun commit_quorum () (_ BitVec 4))
(declare-fun ot_term ((_ BitVec 4)) (_ BitVec 3))
(declare-fun oi_index ((_ BitVec 4)) (_ BitVec 3))
(declare-fun @T1 () (_ BitVec 3))
(declare-fun @I () (_ BitVec 3))
(declare-fun @T2 () (_ BitVec 3))
(declare-fun term_bigger ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun append_reply_msg ((_ BitVec 2) (_ BitVec 3) (_ BitVec 3) (_ BitVec 3)) Bool)
(declare-fun member ((_ BitVec 2) (_ BitVec 4)) Bool)
(declare-fun choosable ((_ BitVec 3) (_ BitVec 3)) Bool)
(declare-fun __ts0_b () Bool)
(declare-fun __started ((_ BitVec 3)) Bool)
(declare-fun started ((_ BitVec 3)) Bool)
(declare-fun __leader ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun leader ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun is_none_term ((_ BitVec 4)) Bool)
(declare-fun is_none_index ((_ BitVec 4)) Bool)
(declare-fun vote_msg ((_ BitVec 2) (_ BitVec 3) (_ BitVec 2)) Bool)
(declare-fun current_term ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun __ts0_a () Bool)
(declare-fun term_at ((_ BitVec 4) (_ BitVec 3) (_ BitVec 3)) Bool)
(declare-fun index_used ((_ BitVec 4) (_ BitVec 3)) Bool)
(declare-fun committed ((_ BitVec 3) (_ BitVec 2)) Bool)
(declare-fun valid ((_ BitVec 4)) Bool)
(declare-fun append_msg ((_ BitVec 3) (_ BitVec 4) (_ BitVec 4) (_ BitVec 3) (_ BitVec 3) (_ BitVec 2)) Bool)
(declare-fun voted ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun request_vote_msg ((_ BitVec 2) (_ BitVec 3) (_ BitVec 4) (_ BitVec 4)) Bool)
(assert (forall ((|N1:node| (_ BitVec 2)) (|N2:node| (_ BitVec 2)) (|T:term| (_ BitVec 3))) (=> (vote_msg |N2:node| |T:term| |N1:node|) (voted |N1:node| |T:term|))))
(assert (forall ((|N1:node| (_ BitVec 2)) (|N2:node| (_ BitVec 2)) (|N3:node| (_ BitVec 2)) (|T:term| (_ BitVec 3))) (=> (and (vote_msg |N2:node| |T:term| |N1:node|) (vote_msg |N3:node| |T:term| |N1:node|)) (= |N2:node| |N3:node|))))
(assert (forall ((|N1:node| (_ BitVec 2)) (|T:term| (_ BitVec 3))) (=> (__leader |N1:node| |T:term|) (forall ((|N2:node| (_ BitVec 2))) (=> (member |N2:node| (__voting_quorum |T:term|)) (vote_msg |N1:node| |T:term| |N2:node|))))))
(assert (forall ((|N:node| (_ BitVec 2)) (|OLI:option_index| (_ BitVec 4)) (|OLT:option_term| (_ BitVec 4)) (|T:term| (_ BitVec 3))) (=> (request_vote_msg |N:node| |T:term| |OLI:option_index| |OLT:option_term|) (= (is_none_index |OLI:option_index|) (is_none_term |OLT:option_term|)))))
(assert (forall ((|C:node| (_ BitVec 2)) (|OLI1:option_index| (_ BitVec 4)) (|OLI2:option_index| (_ BitVec 4)) (|OLT1:option_term| (_ BitVec 4)) (|OLT2:option_term| (_ BitVec 4)) (|T:term| (_ BitVec 3))) (=> (and (request_vote_msg |C:node| |T:term| |OLI1:option_index| |OLT1:option_term|) (request_vote_msg |C:node| |T:term| |OLI2:option_index| |OLT2:option_term|)) (and (= |OLI1:option_index| |OLI2:option_index|) (= |OLT1:option_term| |OLT2:option_term|)))))
(assert (forall ((|C:node| (_ BitVec 2)) (|OLI:option_index| (_ BitVec 4)) (|OLT:option_term| (_ BitVec 4)) (|T:term| (_ BitVec 3))) (=> (request_vote_msg |C:node| |T:term| |OLI:option_index| |OLT:option_term|) (or (current_term |C:node| |T:term|) (term_bigger |C:node| |T:term|)))))
(assert (forall ((|C:node| (_ BitVec 2)) (|OLI:option_index| (_ BitVec 4)) (|OLT:option_term| (_ BitVec 4)) (|T:term| (_ BitVec 3))) (=> (and (and (and (request_vote_msg |C:node| |T:term| |OLI:option_index| |OLT:option_term|) (not (is_none_index |OLI:option_index|))) (not (term_bigger |C:node| |T:term|))) (not (__started |T:term|))) (term_at (current_log |C:node|) (oi_index |OLI:option_index|) (ot_term |OLT:option_term|)))))
(assert (forall ((|C:node| (_ BitVec 2)) (|I:index| (_ BitVec 3)) (|OLI:option_index| (_ BitVec 4)) (|OLT:option_term| (_ BitVec 4)) (|T:term| (_ BitVec 3))) (=> (and (and (and (request_vote_msg |C:node| |T:term| |OLI:option_index| |OLT:option_term|) (is_none_index |OLI:option_index|)) (not (term_bigger |C:node| |T:term|))) (not (__started |T:term|))) (not (index_used (current_log |C:node|) |I:index|)))))
(assert (forall ((|N:node| (_ BitVec 2)) (|T1:term| (_ BitVec 3)) (|T2:term| (_ BitVec 3))) (=> (and (voted |N:node| |T2:term|) (bvult |T1:term| |T2:term|)) (term_bigger |N:node| |T1:term|))))
(assert (forall ((|EI:index| (_ BitVec 3)) (|ET:term| (_ BitVec 3)) (|EV:value| (_ BitVec 2)) (|OPI:option_index| (_ BitVec 4)) (|OPT:option_term| (_ BitVec 4)) (|T:term| (_ BitVec 3))) (=> (append_msg |T:term| |OPI:option_index| |OPT:option_term| |EI:index| |ET:term| |EV:value|) (= (is_none_index |OPI:option_index|) (is_none_term |OPT:option_term|)))))
(assert (forall ((|EI:index| (_ BitVec 3)) (|ET:term| (_ BitVec 3)) (|EV:value| (_ BitVec 2)) (|OPI:option_index| (_ BitVec 4)) (|OPT:option_term| (_ BitVec 4)) (|T:term| (_ BitVec 3))) (=> (and (append_msg |T:term| |OPI:option_index| |OPT:option_term| |EI:index| |ET:term| |EV:value|) (not (is_none_index |OPI:option_index|))) (and (bvult (oi_index |OPI:option_index|) |EI:index|) (forall ((|I:index| (_ BitVec 3))) (=> (bvult |I:index| |EI:index|) (bvule |I:index| (oi_index |OPI:option_index|))))))))
(assert (forall ((|EI:index| (_ BitVec 3)) (|ET:term| (_ BitVec 3)) (|N:node| (_ BitVec 2)) (|T:term| (_ BitVec 3))) (=> (append_reply_msg |N:node| |T:term| |EI:index| |ET:term|) (term_at (__term_log |T:term|) |EI:index| |ET:term|))))
(assert (forall ((|T:term| (_ BitVec 3))) (valid (__term_log |T:term|))))
(assert (valid commit_log))
(assert (forall ((|I1:index| (_ BitVec 3)) (|I2:index| (_ BitVec 3)) (|L:log| (_ BitVec 4))) (=> (and (and (valid |L:log|) (index_used |L:log| |I1:index|)) (bvule |I2:index| |I1:index|)) (index_used |L:log| |I2:index|))))
(assert (forall ((|I:index| (_ BitVec 3)) (|L:log| (_ BitVec 4)) (|T:term| (_ BitVec 3))) (=> (and (valid |L:log|) (term_at |L:log| |I:index| |T:term|)) (index_used |L:log| |I:index|))))
(assert (forall ((|I:index| (_ BitVec 3)) (|L:log| (_ BitVec 4)) (|T:term| (_ BitVec 3))) (=> (and (valid |L:log|) (term_at |L:log| |I:index| |T:term|)) (and (and (index_used (__term_log |T:term|) |I:index|) (term_at (__term_log |T:term|) |I:index| |T:term|)) (= (value_at (__term_log |T:term|) |I:index|) (value_at |L:log| |I:index|))))))
(assert (forall ((|I:index| (_ BitVec 3)) (|L:log| (_ BitVec 4)) (|T1:term| (_ BitVec 3)) (|T2:term| (_ BitVec 3))) (=> (and (term_at |L:log| |I:index| |T1:term|) (term_at |L:log| |I:index| |T2:term|)) (= |T1:term| |T2:term|))))
(assert (forall ((|N:node| (_ BitVec 2)) (|T:term| (_ BitVec 3))) (not (and (term_bigger |N:node| |T:term|) (current_term |N:node| |T:term|)))))
(assert (forall ((|N:node| (_ BitVec 2)) (|T:term| (_ BitVec 3))) (=> (__leader |N:node| |T:term|) (or (current_term |N:node| |T:term|) (term_bigger |N:node| |T:term|)))))
(assert (forall ((|N:node| (_ BitVec 2)) (|T:term| (_ BitVec 3))) (=> (__leader |N:node| |T:term|) (__started |T:term|))))
(assert (forall ((|T:term| (_ BitVec 3))) (=> (__started |T:term|) (__leader (__term_leader |T:term|) |T:term|))))
(assert (forall ((|I:index| (_ BitVec 3)) (|T1:term| (_ BitVec 3)) (|T2:term| (_ BitVec 3))) (=> (term_at (__term_log |T2:term|) |I:index| |T1:term|) (bvule |T1:term| |T2:term|))))
(assert (forall ((|I:index| (_ BitVec 3)) (|T:term| (_ BitVec 3))) (=> (term_at commit_log |I:index| |T:term|) (bvule |T:term| commit_term))))
(assert (forall ((|N:node| (_ BitVec 2)) (|T:term| (_ BitVec 3))) (=> (and (__leader |N:node| |T:term|) (current_term |N:node| |T:term|)) (= (current_log |N:node|) (__term_log |T:term|)))))
(assert (forall ((|C:node| (_ BitVec 2)) (|I:index| (_ BitVec 3)) (|N:node| (_ BitVec 2)) (|T1:term| (_ BitVec 3)) (|T2:term| (_ BitVec 3))) (=> (and (and (and (and (and (choosable |I:index| |T1:term|) (append_reply_msg |N:node| |T1:term| |I:index| |T1:term|)) (vote_msg |C:node| |T2:term| |N:node|)) (bvugt |T2:term| |T1:term|)) (not (__started |T2:term|))) (not (term_bigger |C:node| |T2:term|))) (term_at (current_log |C:node|) |I:index| |T1:term|))))
(assert (forall ((|I:index| (_ BitVec 3)) (|T1:term| (_ BitVec 3)) (|T2:term| (_ BitVec 3))) (=> (and (and (and (term_at (__term_log |T1:term|) |I:index| |T1:term|) (choosable |I:index| |T1:term|)) (__started |T2:term|)) (bvule |T1:term| |T2:term|)) (term_at (__term_log |T2:term|) |I:index| |T1:term|))))
(assert (or __ts0_a __ts0_b))
(assert (or (not __ts0_a) (current_term |__fml:n| |__loc:t|)))
(assert (forall ((|N:node| (_ BitVec 2))) (or (not __ts0_a) (=> (member |N:node| |__loc:q|) (vote_msg |__fml:n| |__loc:t| |N:node|)))))
(assert (forall ((|Q1:quorum| (_ BitVec 4)) (|Q2:quorum| (_ BitVec 4))) (exists ((|N:node| (_ BitVec 2))) (and (member |N:node| |Q1:quorum|) (member |N:node| |Q2:quorum|)))))
(assert (forall ((|V0:term| (_ BitVec 3))) (= (started |V0:term|) (ite __ts0_b (__started |V0:term|) (ite (and (= |V0:term| |__loc:t|)) true (__started |V0:term|))))))
(assert (forall ((|V0:term| (_ BitVec 3))) (= (term_log |V0:term|) (ite __ts0_b (__term_log |V0:term|) (ite (and (= |V0:term| |__loc:t|)) (current_log |__fml:n|) (__term_log |V0:term|))))))
(assert (forall ((|I:index| (_ BitVec 3)) (|T:term| (_ BitVec 3))) (= (choosable |I:index| |T:term|) (exists ((|Q:quorum| (_ BitVec 4))) (forall ((|N:node| (_ BitVec 2))) (=> (member |N:node| |Q:quorum|) (or (append_reply_msg |N:node| |T:term| |I:index| |T:term|) (not (term_bigger |N:node| |T:term|)))))))))
(assert (not (=> (and (and (and (term_at (term_log @T1) @I @T1) (choosable @I @T1)) (started @T2)) (bvule @T1 @T2)) (term_at (term_log @T2) @I @T1))))
(check-sat)

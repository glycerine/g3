; Verification condition 274 emitted by Ivy for examples/raft/raft_no_assume_test.ivy.
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

(declare-fun term_bigger ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun append_reply_msg
             ((_ BitVec 2) (_ BitVec 3) (_ BitVec 3) (_ BitVec 3))
             Bool)
(declare-fun member ((_ BitVec 2) (_ BitVec 4)) Bool)
(declare-fun choosable ((_ BitVec 3) (_ BitVec 3)) Bool)
(declare-fun __append_reply_msg
             ((_ BitVec 2) (_ BitVec 3) (_ BitVec 3) (_ BitVec 3))
             Bool)
(declare-fun __ts0__m_choosable ((_ BitVec 3) (_ BitVec 3)) Bool)
(declare-fun is_none_index ((_ BitVec 4)) Bool)
(declare-fun |__loc:opi| () (_ BitVec 4))
(declare-fun __ts0__ts0_g () Bool)
(declare-fun __term_at ((_ BitVec 4) (_ BitVec 3) (_ BitVec 3)) Bool)
(declare-fun |__ts0__ts0__new_loc:pt_a| () (_ BitVec 3))
(declare-fun |__ts0__ts0__new_loc:pi_a| () (_ BitVec 3))
(declare-fun __current_log ((_ BitVec 2)) (_ BitVec 4))
(declare-fun |__fml:n| () (_ BitVec 2))
(declare-fun __ts0__ts0__ts0_f_a () Bool)
(declare-fun __index_used ((_ BitVec 4) (_ BitVec 3)) Bool)
(declare-fun |__loc:ei| () (_ BitVec 3))
(declare-fun __ts0__ts0__ts0__ts0_e_a () Bool)
(declare-fun ot_term ((_ BitVec 4)) (_ BitVec 3))
(declare-fun |__loc:opt| () (_ BitVec 4))
(declare-fun oi_index ((_ BitVec 4)) (_ BitVec 3))
(declare-fun __ts0__ts0__ts0_e_a_a () Bool)
(declare-fun |__loc:et| () (_ BitVec 3))
(declare-fun |__loc:t2| () (_ BitVec 3))
(declare-fun __ts0_b () Bool)
(declare-fun __ts0_d () Bool)
(declare-fun __ts0_f () Bool)
(declare-fun |__loc:l2| () (_ BitVec 4))
(declare-fun current_log ((_ BitVec 2)) (_ BitVec 4))
(declare-fun __valid ((_ BitVec 4)) Bool)
(declare-fun valid ((_ BitVec 4)) Bool)
(declare-fun __value_at ((_ BitVec 4) (_ BitVec 3)) (_ BitVec 2))
(declare-fun |__loc:ev| () (_ BitVec 2))
(declare-fun value_at ((_ BitVec 4) (_ BitVec 3)) (_ BitVec 2))
(declare-fun term_at ((_ BitVec 4) (_ BitVec 3) (_ BitVec 3)) Bool)
(declare-fun index_used ((_ BitVec 4) (_ BitVec 3)) Bool)
(declare-fun __term_bigger ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun |__ts0__new_fml:t| () (_ BitVec 3))
(declare-fun |__ts0__new_fml:n_a| () (_ BitVec 2))
(declare-fun __current_term ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun current_term ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun __choosable ((_ BitVec 3) (_ BitVec 3)) Bool)
(declare-fun none_term () (_ BitVec 4))
(declare-fun is_none_term ((_ BitVec 4)) Bool)
(declare-fun none_index () (_ BitVec 4))
(declare-fun append_msg
             ((_ BitVec 3)
              (_ BitVec 4)
              (_ BitVec 4)
              (_ BitVec 3)
              (_ BitVec 3)
              (_ BitVec 2))
             Bool)
(declare-fun __ts0_e () Bool)
(declare-fun __ts0_c () Bool)
(declare-fun __ts0_a () Bool)
(declare-fun |__loc:ti| () (_ BitVec 3))
(declare-fun |__loc:t| () (_ BitVec 3))
(declare-fun term_log ((_ BitVec 3)) (_ BitVec 4))
(declare-fun started ((_ BitVec 3)) Bool)
(declare-fun vote_msg ((_ BitVec 2) (_ BitVec 3) (_ BitVec 2)) Bool)
(declare-fun commit_term () (_ BitVec 3))
(declare-fun commit_index () (_ BitVec 3))
(declare-fun commit_log () (_ BitVec 4))
(declare-fun committed ((_ BitVec 3) (_ BitVec 2)) Bool)
(declare-fun commit_value () (_ BitVec 2))
(declare-fun commit_quorum () (_ BitVec 4))
(declare-fun leader ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun term_leader ((_ BitVec 3)) (_ BitVec 2))
(declare-fun voted ((_ BitVec 2) (_ BitVec 3)) Bool)
(declare-fun request_vote_msg
             ((_ BitVec 2) (_ BitVec 3) (_ BitVec 4) (_ BitVec 4))
             Bool)
(declare-fun voting_quorum ((_ BitVec 3)) (_ BitVec 4))
(declare-fun @T1 () (_ BitVec 3))
(declare-fun @I () (_ BitVec 3))
(declare-fun @N () (_ BitVec 2))
(assert (let ((a!1 (forall ((|N1:node| (_ BitVec 2))
                    (|N2:node| (_ BitVec 2))
                    (|N3:node| (_ BitVec 2))
                    (|T:term| (_ BitVec 3)))
             (=> (and (vote_msg |N2:node| |T:term| |N1:node|)
                      (vote_msg |N3:node| |T:term| |N1:node|))
                 (= |N2:node| |N3:node|))))
      (a!2 (forall ((|N1:node| (_ BitVec 2))
                    (|N2:node| (_ BitVec 2))
                    (|T:term| (_ BitVec 3)))
             (=> (and (leader |N1:node| |T:term|) (leader |N2:node| |T:term|))
                 (= |N1:node| |N2:node|))))
      (a!3 (forall ((|N:node| (_ BitVec 2))
                    (|OLI:option_index| (_ BitVec 4))
                    (|OLT:option_term| (_ BitVec 4))
                    (|T:term| (_ BitVec 3)))
             (=> (request_vote_msg |N:node|
                                   |T:term|
                                   |OLI:option_index|
                                   |OLT:option_term|)
                 (= (is_none_index |OLI:option_index|)
                    (is_none_term |OLT:option_term|)))))
      (a!4 (forall ((|C:node| (_ BitVec 2))
                    (|OLI1:option_index| (_ BitVec 4))
                    (|OLI2:option_index| (_ BitVec 4))
                    (|OLT1:option_term| (_ BitVec 4))
                    (|OLT2:option_term| (_ BitVec 4))
                    (|T:term| (_ BitVec 3)))
             (=> (and (request_vote_msg |C:node|
                                        |T:term|
                                        |OLI1:option_index|
                                        |OLT1:option_term|)
                      (request_vote_msg |C:node|
                                        |T:term|
                                        |OLI2:option_index|
                                        |OLT2:option_term|))
                 (and (= |OLI1:option_index| |OLI2:option_index|)
                      (= |OLT1:option_term| |OLT2:option_term|)))))
      (a!5 (forall ((|C:node| (_ BitVec 2))
                    (|OLI:option_index| (_ BitVec 4))
                    (|OLT:option_term| (_ BitVec 4))
                    (|T:term| (_ BitVec 3)))
             (=> (request_vote_msg |C:node|
                                   |T:term|
                                   |OLI:option_index|
                                   |OLT:option_term|)
                 (or (__current_term |C:node| |T:term|)
                     (__term_bigger |C:node| |T:term|)))))
      (a!6 (forall ((|C:node| (_ BitVec 2))
                    (|OLI:option_index| (_ BitVec 4))
                    (|OLT:option_term| (_ BitVec 4))
                    (|T:term| (_ BitVec 3)))
             (=> (and (request_vote_msg |C:node|
                                        |T:term|
                                        |OLI:option_index|
                                        |OLT:option_term|)
                      (not (is_none_index |OLI:option_index|))
                      (not (__term_bigger |C:node| |T:term|))
                      (not (started |T:term|)))
                 (__term_at (__current_log |C:node|)
                            (oi_index |OLI:option_index|)
                            (ot_term |OLT:option_term|)))))
      (a!7 (forall ((|C:node| (_ BitVec 2))
                    (|I:index| (_ BitVec 3))
                    (|OLI:option_index| (_ BitVec 4))
                    (|OLT:option_term| (_ BitVec 4))
                    (|T:term| (_ BitVec 3)))
             (=> (and (request_vote_msg |C:node|
                                        |T:term|
                                        |OLI:option_index|
                                        |OLT:option_term|)
                      (not (is_none_index |OLI:option_index|))
                      (not (__term_bigger |C:node| |T:term|))
                      (not (started |T:term|))
                      (bvugt |I:index| (oi_index |OLI:option_index|)))
                 (not (__index_used (__current_log |C:node|) |I:index|)))))
      (a!8 (forall ((|C:node| (_ BitVec 2))
                    (|I:index| (_ BitVec 3))
                    (|OLI:option_index| (_ BitVec 4))
                    (|OLT:option_term| (_ BitVec 4))
                    (|T:term| (_ BitVec 3)))
             (=> (and (request_vote_msg |C:node|
                                        |T:term|
                                        |OLI:option_index|
                                        |OLT:option_term|)
                      (is_none_index |OLI:option_index|)
                      (not (__term_bigger |C:node| |T:term|))
                      (not (started |T:term|)))
                 (not (__index_used (__current_log |C:node|) |I:index|)))))
      (a!9 (forall ((|N:node| (_ BitVec 2))
                    (|T1:term| (_ BitVec 3))
                    (|T2:term| (_ BitVec 3)))
             (=> (and (voted |N:node| |T2:term|) (bvult |T1:term| |T2:term|))
                 (__term_bigger |N:node| |T1:term|))))
      (a!10 (forall ((|EI:index| (_ BitVec 3))
                     (|ET:term| (_ BitVec 3))
                     (|EV:value| (_ BitVec 2))
                     (|OPI:option_index| (_ BitVec 4))
                     (|OPT:option_term| (_ BitVec 4))
                     (|T:term| (_ BitVec 3)))
              (=> (append_msg |T:term|
                              |OPI:option_index|
                              |OPT:option_term|
                              |EI:index|
                              |ET:term|
                              |EV:value|)
                  (= (is_none_index |OPI:option_index|)
                     (is_none_term |OPT:option_term|)))))
      (a!11 (forall ((|EI:index| (_ BitVec 3))
                     (|ET:term| (_ BitVec 3))
                     (|EV:value| (_ BitVec 2))
                     (|OPI:option_index| (_ BitVec 4))
                     (|OPT:option_term| (_ BitVec 4))
                     (|T:term| (_ BitVec 3)))
              (=> (and (append_msg |T:term|
                                   |OPI:option_index|
                                   |OPT:option_term|
                                   |EI:index|
                                   |ET:term|
                                   |EV:value|)
                       (is_none_index |OPI:option_index|))
                  (= |EI:index| #b000))))
      (a!12 (forall ((|EI:index| (_ BitVec 3))
                     (|ET:term| (_ BitVec 3))
                     (|EV:value| (_ BitVec 2))
                     (|OPI:option_index| (_ BitVec 4))
                     (|OPT:option_term| (_ BitVec 4))
                     (|T:term| (_ BitVec 3)))
              (let ((a!1 (and (append_msg |T:term|
                                          |OPI:option_index|
                                          |OPT:option_term|
                                          |EI:index|
                                          |ET:term|
                                          |EV:value|)
                              (not (is_none_index |OPI:option_index|))))
                    (a!2 (forall ((|I:index| (_ BitVec 3)))
                           (=> (bvult |I:index| |EI:index|)
                               (bvule |I:index| (oi_index |OPI:option_index|))))))
                (=> a!1
                    (and (bvult (oi_index |OPI:option_index|) |EI:index|) a!2)))))
      (a!13 (forall ((|EI:index| (_ BitVec 3))
                     (|ET:term| (_ BitVec 3))
                     (|EV:value| (_ BitVec 2))
                     (|PLI:option_index| (_ BitVec 4))
                     (|PLT:option_term| (_ BitVec 4))
                     (|T:term| (_ BitVec 3)))
              (let ((a!1 (and (append_msg |T:term|
                                          |PLI:option_index|
                                          |PLT:option_term|
                                          |EI:index|
                                          |ET:term|
                                          |EV:value|)
                              (not (is_none_index |PLI:option_index|)))))
                (=> a!1
                    (__term_at (term_log |T:term|)
                               (oi_index |PLI:option_index|)
                               (ot_term |PLT:option_term|))))))
      (a!14 (forall ((|EI:index| (_ BitVec 3))
                     (|ET:term| (_ BitVec 3))
                     (|N:node| (_ BitVec 2))
                     (|T:term| (_ BitVec 3)))
              (=> (__append_reply_msg |N:node| |T:term| |EI:index| |ET:term|)
                  (__term_at (term_log |T:term|) |EI:index| |ET:term|))))
      (a!15 (forall ((|I1:index| (_ BitVec 3))
                     (|I2:index| (_ BitVec 3))
                     (|L1:log| (_ BitVec 4))
                     (|L2:log| (_ BitVec 4))
                     (|T1:term| (_ BitVec 3))
                     (|T2:term| (_ BitVec 3)))
              (=> (and (__valid |L1:log|)
                       (__valid |L2:log|)
                       (__index_used |L1:log| |I1:index|)
                       (__term_at |L1:log| |I1:index| |T1:term|)
                       (__index_used |L2:log| |I1:index|)
                       (__term_at |L2:log| |I1:index| |T1:term|)
                       (bvule |I2:index| |I1:index|))
                  (and (= (__term_at |L1:log| |I2:index| |T2:term|)
                          (__term_at |L2:log| |I2:index| |T2:term|))
                       (= (__value_at |L1:log| |I2:index|)
                          (__value_at |L2:log| |I2:index|))))))
      (a!16 (forall ((|I1:index| (_ BitVec 3))
                     (|I2:index| (_ BitVec 3))
                     (|L:log| (_ BitVec 4)))
              (=> (and (__valid |L:log|)
                       (__index_used |L:log| |I1:index|)
                       (bvule |I2:index| |I1:index|))
                  (__index_used |L:log| |I2:index|))))
      (a!17 (forall ((|I:index| (_ BitVec 3))
                     (|L:log| (_ BitVec 4))
                     (|T:term| (_ BitVec 3)))
              (=> (and (__valid |L:log|) (__term_at |L:log| |I:index| |T:term|))
                  (__index_used |L:log| |I:index|))))
      (a!18 (forall ((|I:index| (_ BitVec 3))
                     (|L:log| (_ BitVec 4))
                     (|T:term| (_ BitVec 3)))
              (let ((a!1 (and (__index_used (term_log |T:term|) |I:index|)
                              (__term_at (term_log |T:term|) |I:index| |T:term|)
                              (= (__value_at (term_log |T:term|) |I:index|)
                                 (__value_at |L:log| |I:index|)))))
                (=> (and (__valid |L:log|)
                         (__term_at |L:log| |I:index| |T:term|))
                    a!1))))
      (a!19 (forall ((|I:index| (_ BitVec 3))
                     (|L:log| (_ BitVec 4))
                     (|T1:term| (_ BitVec 3))
                     (|T2:term| (_ BitVec 3)))
              (=> (and (__term_at |L:log| |I:index| |T1:term|)
                       (__term_at |L:log| |I:index| |T2:term|))
                  (= |T1:term| |T2:term|))))
      (a!20 (forall ((|N:node| (_ BitVec 2))
                     (|T1:term| (_ BitVec 3))
                     (|T2:term| (_ BitVec 3)))
              (=> (and (__current_term |N:node| |T1:term|)
                       (__current_term |N:node| |T2:term|))
                  (= |T1:term| |T2:term|))))
      (a!21 (forall ((|N:node| (_ BitVec 2))
                     (|T1:term| (_ BitVec 3))
                     (|T2:term| (_ BitVec 3)))
              (=> (and (__current_term |N:node| |T1:term|)
                       (bvult |T2:term| |T1:term|))
                  (__term_bigger |N:node| |T2:term|))))
      (a!22 (forall ((|N:node| (_ BitVec 2))
                     (|T1:term| (_ BitVec 3))
                     (|T2:term| (_ BitVec 3)))
              (=> (and (__term_bigger |N:node| |T1:term|)
                       (bvult |T2:term| |T1:term|))
                  (__term_bigger |N:node| |T2:term|))))
      (a!23 (forall ((|N:node| (_ BitVec 2)) (|T:term| (_ BitVec 3)))
              (not (and (__term_bigger |N:node| |T:term|)
                        (__current_term |N:node| |T:term|)))))
      (a!24 (forall ((|N:node| (_ BitVec 2)) (|T:term| (_ BitVec 3)))
              (=> (leader |N:node| |T:term|)
                  (or (__current_term |N:node| |T:term|)
                      (__term_bigger |N:node| |T:term|)))))
      (a!25 (forall ((|T:term| (_ BitVec 3)))
              (=> (started |T:term|) (leader (term_leader |T:term|) |T:term|))))
      (a!26 (forall ((|I:index| (_ BitVec 3))
                     (|N:node| (_ BitVec 2))
                     (|T:term| (_ BitVec 3)))
              (=> (__term_at (__current_log |N:node|) |I:index| |T:term|)
                  (or (__current_term |N:node| |T:term|)
                      (__term_bigger |N:node| |T:term|)))))
      (a!27 (forall ((|I:index| (_ BitVec 3))
                     (|T1:term| (_ BitVec 3))
                     (|T2:term| (_ BitVec 3)))
              (=> (__term_at (term_log |T2:term|) |I:index| |T1:term|)
                  (bvule |T1:term| |T2:term|))))
      (a!28 (forall ((|N:node| (_ BitVec 2)) (|T:term| (_ BitVec 3)))
              (=> (and (leader |N:node| |T:term|)
                       (__current_term |N:node| |T:term|))
                  (= (__current_log |N:node|) (term_log |T:term|)))))
      (a!29 (forall ((|I:index| (_ BitVec 3)) (|T:term| (_ BitVec 3)))
              (=> (__index_used (term_log |T:term|) |I:index|)
                  (started |T:term|))))
      (a!30 (=> (committed commit_index commit_value)
                (forall ((|N:node| (_ BitVec 2)))
                  (=> (member |N:node| commit_quorum)
                      (__append_reply_msg
                        |N:node|
                        commit_term
                        commit_index
                        commit_term)))))
      (a!31 (forall ((|I:index| (_ BitVec 3)))
              (=> (committed commit_index commit_value)
                  (= (__index_used commit_log |I:index|)
                     (bvule |I:index| commit_index)))))
      (a!32 (forall ((|I:index| (_ BitVec 3)) (|V:value| (_ BitVec 2)))
              (= (committed |I:index| |V:value|)
                 (and (__index_used commit_log |I:index|)
                      (= |V:value| (__value_at commit_log |I:index|))))))
      (a!33 (forall ((|I:index| (_ BitVec 3))
                     (|N:node| (_ BitVec 2))
                     (|T1:term| (_ BitVec 3)))
              (=> (and (__choosable |I:index| |T1:term|)
                       (__append_reply_msg
                         |N:node|
                         |T1:term|
                         |I:index|
                         |T1:term|))
                  (__term_at (__current_log |N:node|) |I:index| |T1:term|))))
      (a!34 (forall ((|C:node| (_ BitVec 2))
                     (|I:index| (_ BitVec 3))
                     (|N:node| (_ BitVec 2))
                     (|T1:term| (_ BitVec 3))
                     (|T2:term| (_ BitVec 3)))
              (=> (and (__choosable |I:index| |T1:term|)
                       (__append_reply_msg
                         |N:node|
                         |T1:term|
                         |I:index|
                         |T1:term|)
                       (vote_msg |C:node| |T2:term| |N:node|)
                       (bvugt |T2:term| |T1:term|)
                       (not (started |T2:term|))
                       (not (__term_bigger |C:node| |T2:term|)))
                  (__term_at (__current_log |C:node|) |I:index| |T1:term|))))
      (a!35 (forall ((|I:index| (_ BitVec 3))
                     (|T1:term| (_ BitVec 3))
                     (|T2:term| (_ BitVec 3)))
              (=> (and (__term_at (term_log |T1:term|) |I:index| |T1:term|)
                       (__choosable |I:index| |T1:term|)
                       (started |T2:term|)
                       (bvule |T1:term| |T2:term|))
                  (__term_at (term_log |T2:term|) |I:index| |T1:term|))))
      (a!36 (or (not __ts0_e)
                (not __ts0__ts0_g)
                (not __ts0__ts0__ts0_e_a_a)
                (or (not __ts0_a) (not (__valid |__loc:l2|)))))
      (a!37 (or (not __ts0_e)
                (not __ts0__ts0_g)
                __ts0__ts0__ts0_e_a_a
                (or (not __ts0_c)
                    (__term_at (__current_log |__fml:n|) |__loc:ei| |__loc:ti|))))
      (a!38 (or (not __ts0_e)
                (not __ts0__ts0_g)
                __ts0__ts0__ts0_e_a_a
                (or (not __ts0_c) (not (= |__loc:et| |__loc:ti|)))))
      (a!39 (or (not __ts0_c) (or (not __ts0_a) (not (__valid |__loc:l2|)))))
      (a!40 (forall ((|V0:log| (_ BitVec 4)))
              (let ((a!1 (or (not __ts0_b) (not (not (__valid |V0:log|))))))
                (or (not __ts0_e)
                    (not __ts0__ts0_g)
                    __ts0__ts0__ts0_e_a_a
                    (or (not __ts0_c) a!1)))))
      (a!41 (forall ((|V0:term| (_ BitVec 3)))
              (let ((a!1 (not (and (__term_at (__current_log |__fml:n|)
                                              |__loc:ei|
                                              |V0:term|)
                                   (not (= |__loc:et| |V0:term|))))))
                (or (not __ts0_e)
                    (not __ts0__ts0_g)
                    __ts0__ts0__ts0_e_a_a
                    (or (not __ts0_d) a!1)))))
      (a!42 (or (not __ts0_e)
                __ts0__ts0_g
                (not __ts0__ts0__ts0_f_a)
                (not __ts0__ts0__ts0__ts0_e_a)
                (or (not __ts0_a) (not (__valid |__loc:l2|)))))
      (a!43 (or (not __ts0_e)
                __ts0__ts0_g
                (not __ts0__ts0__ts0_f_a)
                __ts0__ts0__ts0__ts0_e_a
                (or (not __ts0_c)
                    (__term_at (__current_log |__fml:n|) |__loc:ei| |__loc:ti|))))
      (a!44 (or (not __ts0_e)
                __ts0__ts0_g
                (not __ts0__ts0__ts0_f_a)
                __ts0__ts0__ts0__ts0_e_a
                (or (not __ts0_c) (not (= |__loc:et| |__loc:ti|)))))
      (a!45 (forall ((|V0:log| (_ BitVec 4)))
              (let ((a!1 (or (not __ts0_b) (not (not (__valid |V0:log|))))))
                (or (not __ts0_e)
                    __ts0__ts0_g
                    (not __ts0__ts0__ts0_f_a)
                    __ts0__ts0__ts0__ts0_e_a
                    (or (not __ts0_c) a!1)))))
      (a!46 (forall ((|V0:term| (_ BitVec 3)))
              (let ((a!1 (not (and (__term_at (__current_log |__fml:n|)
                                              |__loc:ei|
                                              |V0:term|)
                                   (not (= |__loc:et| |V0:term|))))))
                (or (not __ts0_e)
                    __ts0__ts0_g
                    (not __ts0__ts0__ts0_f_a)
                    __ts0__ts0__ts0__ts0_e_a
                    (or (not __ts0_d) a!1)))))
      (a!47 (forall ((|V0:term| (_ BitVec 3))
                     (|V1:term| (_ BitVec 3))
                     (|V2:option_index| (_ BitVec 4))
                     (|V3:option_term| (_ BitVec 4))
                     (|V4:index| (_ BitVec 3))
                     (|V5:term| (_ BitVec 3))
                     (|V6:value| (_ BitVec 2)))
              (or (not __ts0_f)
                  (not (and (__current_term |__fml:n| |V0:term|)
                            (append_msg |V1:term|
                                        |V2:option_index|
                                        |V3:option_term|
                                        |V4:index|
                                        |V5:term|
                                        |V6:value|)
                            (bvuge |V1:term| |V0:term|))))))
      (a!48 (forall ((|X:term| (_ BitVec 3))
                     (|Y:term| (_ BitVec 3))
                     (|Z:term| (_ BitVec 3)))
              (=> (and (bvule |X:term| |Y:term|) (bvule |Y:term| |Z:term|))
                  (bvule |X:term| |Z:term|))))
      (a!49 (forall ((|X:term| (_ BitVec 3)) (|Y:term| (_ BitVec 3)))
              (=> (and (bvule |X:term| |Y:term|) (bvule |Y:term| |X:term|))
                  (= |X:term| |Y:term|))))
      (a!50 (forall ((|X:term| (_ BitVec 3)) (|Y:term| (_ BitVec 3)))
              (= (bvult |X:term| |Y:term|)
                 (and (bvule |X:term| |Y:term|) (not (= |X:term| |Y:term|))))))
      (a!51 (forall ((|X:index| (_ BitVec 3))
                     (|Y:index| (_ BitVec 3))
                     (|Z:index| (_ BitVec 3)))
              (=> (and (bvule |X:index| |Y:index|) (bvule |Y:index| |Z:index|))
                  (bvule |X:index| |Z:index|))))
      (a!52 (forall ((|X:index| (_ BitVec 3)) (|Y:index| (_ BitVec 3)))
              (=> (and (bvule |X:index| |Y:index|) (bvule |Y:index| |X:index|))
                  (= |X:index| |Y:index|))))
      (a!53 (forall ((|X:index| (_ BitVec 3)) (|Y:index| (_ BitVec 3)))
              (= (bvult |X:index| |Y:index|)
                 (and (bvule |X:index| |Y:index|) (not (= |X:index| |Y:index|))))))
      (a!54 (forall ((|Q1:quorum| (_ BitVec 4)) (|Q2:quorum| (_ BitVec 4)))
              (exists ((|N:node| (_ BitVec 2)))
                (and (member |N:node| |Q1:quorum|)
                     (member |N:node| |Q2:quorum|)))))
      (a!55 (forall ((|V0:log| (_ BitVec 4)) (|V1:index| (_ BitVec 3)))
              (let ((a!1 (ite __ts0_b
                              (__index_used |V0:log| |V1:index|)
                              (ite (and (= |V0:log| |__loc:l2|))
                                   (= |V1:index| |__loc:ei|)
                                   (__index_used |V0:log| |V1:index|))))
                    (a!2 (ite __ts0_b
                              (__index_used |V0:log| |V1:index|)
                              (ite (and (= |V0:log| |__loc:l2|))
                                   (bvule |V1:index| |__loc:ei|)
                                   (__index_used |V0:log| |V1:index|)))))
              (let ((a!3 (ite __ts0__ts0__ts0_f_a
                              (ite __ts0__ts0__ts0__ts0_e_a
                                   a!2
                                   (ite __ts0_d
                                        (__index_used |V0:log| |V1:index|)
                                        a!2))
                              (__index_used |V0:log| |V1:index|))))
              (let ((a!4 (ite __ts0__ts0_g
                              (ite __ts0__ts0__ts0_e_a_a
                                   a!1
                                   (ite __ts0_d
                                        (__index_used |V0:log| |V1:index|)
                                        a!1))
                              a!3)))
                (= (index_used |V0:log| |V1:index|)
                   (ite __ts0_f (__index_used |V0:log| |V1:index|) a!4)))))))
      (a!56 (forall ((|V0:log| (_ BitVec 4))
                     (|V1:index| (_ BitVec 3))
                     (|V2:term| (_ BitVec 3)))
              (let ((a!1 (ite __ts0_b
                              (__term_at |V0:log| |V1:index| |V2:term|)
                              (ite (and (= |V0:log| |__loc:l2|))
                                   (and (= |V1:index| |__loc:ei|)
                                        (= |V2:term| |__loc:et|))
                                   (__term_at |V0:log| |V1:index| |V2:term|))))
                    (a!2 (or (and (bvult |V1:index| |__loc:ei|)
                                  (__term_at (__current_log |__fml:n|)
                                             |V1:index|
                                             |V2:term|))
                             (and (= |V1:index| |__loc:ei|)
                                  (= |V2:term| |__loc:et|)))))
              (let ((a!3 (ite __ts0_b
                              (__term_at |V0:log| |V1:index| |V2:term|)
                              (ite (and (= |V0:log| |__loc:l2|))
                                   a!2
                                   (__term_at |V0:log| |V1:index| |V2:term|)))))
              (let ((a!4 (ite __ts0__ts0__ts0_f_a
                              (ite __ts0__ts0__ts0__ts0_e_a
                                   a!3
                                   (ite __ts0_d
                                        (__term_at |V0:log|
                                                   |V1:index|
                                                   |V2:term|)
                                        a!3))
                              (__term_at |V0:log| |V1:index| |V2:term|))))
              (let ((a!5 (ite __ts0__ts0_g
                              (ite __ts0__ts0__ts0_e_a_a
                                   a!1
                                   (ite __ts0_d
                                        (__term_at |V0:log|
                                                   |V1:index|
                                                   |V2:term|)
                                        a!1))
                              a!4)))
                (= (term_at |V0:log| |V1:index| |V2:term|)
                   (ite __ts0_f (__term_at |V0:log| |V1:index| |V2:term|) a!5))))))))
      (a!57 (forall ((|V0:log| (_ BitVec 4)) (|V1:index| (_ BitVec 3)))
              (let ((a!1 (ite __ts0_b
                              (__value_at |V0:log| |V1:index|)
                              (ite (and (= |V0:log| |__loc:l2|))
                                   |__loc:ev|
                                   (__value_at |V0:log| |V1:index|))))
                    (a!2 (ite (and (= |V0:log| |__loc:l2|))
                              (ite (bvult |V1:index| |__loc:ei|)
                                   (__value_at (__current_log |__fml:n|)
                                               |V1:index|)
                                   |__loc:ev|)
                              (__value_at |V0:log| |V1:index|))))
              (let ((a!3 (ite __ts0__ts0__ts0__ts0_e_a
                              (ite __ts0_b (__value_at |V0:log| |V1:index|) a!2)
                              (ite __ts0_d
                                   (__value_at |V0:log| |V1:index|)
                                   (ite __ts0_b
                                        (__value_at |V0:log| |V1:index|)
                                        a!2)))))
              (let ((a!4 (ite __ts0__ts0_g
                              (ite __ts0__ts0__ts0_e_a_a
                                   a!1
                                   (ite __ts0_d
                                        (__value_at |V0:log| |V1:index|)
                                        a!1))
                              (ite __ts0__ts0__ts0_f_a
                                   a!3
                                   (__value_at |V0:log| |V1:index|)))))
                (= (value_at |V0:log| |V1:index|)
                   (ite __ts0_f (__value_at |V0:log| |V1:index|) a!4)))))))
      (a!58 (forall ((|V0:log| (_ BitVec 4)))
              (let ((a!1 (ite __ts0_b
                              (__valid |V0:log|)
                              (ite (and (= |V0:log| |__loc:l2|))
                                   true
                                   (__valid |V0:log|)))))
              (let ((a!2 (ite __ts0__ts0__ts0_f_a
                              (ite __ts0__ts0__ts0__ts0_e_a
                                   a!1
                                   (ite __ts0_d (__valid |V0:log|) a!1))
                              (__valid |V0:log|))))
              (let ((a!3 (ite __ts0__ts0_g
                              (ite __ts0__ts0__ts0_e_a_a
                                   a!1
                                   (ite __ts0_d (__valid |V0:log|) a!1))
                              a!2)))
                (= (valid |V0:log|) (ite __ts0_f (__valid |V0:log|) a!3)))))))
      (a!59 (forall ((|V0:node| (_ BitVec 2)))
              (let ((a!1 (ite __ts0_b
                              (__current_log |V0:node|)
                              (ite (and (= |V0:node| |__fml:n|))
                                   |__loc:l2|
                                   (__current_log |V0:node|)))))
              (let ((a!2 (ite __ts0__ts0__ts0_f_a
                              (ite __ts0__ts0__ts0__ts0_e_a
                                   a!1
                                   (ite __ts0_d (__current_log |V0:node|) a!1))
                              (__current_log |V0:node|))))
              (let ((a!3 (ite __ts0__ts0_g
                              (ite __ts0__ts0__ts0_e_a_a
                                   a!1
                                   (ite __ts0_d (__current_log |V0:node|) a!1))
                              a!2)))
                (= (current_log |V0:node|)
                   (ite __ts0_f (__current_log |V0:node|) a!3)))))))
      (a!60 (forall ((|V0:node| (_ BitVec 2))
                     (|V1:term| (_ BitVec 3))
                     (|V2:index| (_ BitVec 3))
                     (|V3:term| (_ BitVec 3)))
              (let ((a!1 (ite __ts0_b
                              (__append_reply_msg
                                |V0:node|
                                |V1:term|
                                |V2:index|
                                |V3:term|)
                              (ite (and (= |V0:node| |__fml:n|)
                                        (= |V1:term| |__loc:t2|)
                                        (= |V2:index| |__loc:ei|)
                                        (= |V3:term| |__loc:et|))
                                   true
                                   (__append_reply_msg
                                     |V0:node|
                                     |V1:term|
                                     |V2:index|
                                     |V3:term|)))))
              (let ((a!2 (ite __ts0__ts0__ts0_f_a
                              (ite __ts0__ts0__ts0__ts0_e_a
                                   a!1
                                   (ite __ts0_d
                                        (__append_reply_msg
                                          |V0:node|
                                          |V1:term|
                                          |V2:index|
                                          |V3:term|)
                                        a!1))
                              (__append_reply_msg
                                |V0:node|
                                |V1:term|
                                |V2:index|
                                |V3:term|))))
              (let ((a!3 (ite __ts0__ts0_g
                              (ite __ts0__ts0__ts0_e_a_a
                                   a!1
                                   (ite __ts0_d
                                        (__append_reply_msg
                                          |V0:node|
                                          |V1:term|
                                          |V2:index|
                                          |V3:term|)
                                        a!1))
                              a!2)))
                (= (append_reply_msg |V0:node| |V1:term| |V2:index| |V3:term|)
                   (ite __ts0_f
                        (__append_reply_msg
                          |V0:node|
                          |V1:term|
                          |V2:index|
                          |V3:term|)
                        a!3)))))))
      (a!61 (= __ts0__ts0__ts0_e_a_a
               (not (__index_used (__current_log |__fml:n|) |__loc:ei|))))
      (a!62 (= __ts0__ts0__ts0__ts0_e_a
               (not (__index_used (__current_log |__fml:n|) |__loc:ei|)))))
  (and (forall ((|N1:node| (_ BitVec 2))
                (|N2:node| (_ BitVec 2))
                (|T:term| (_ BitVec 3)))
         (=> (vote_msg |N2:node| |T:term| |N1:node|) (voted |N1:node| |T:term|)))
       a!1
       (forall ((|N1:node| (_ BitVec 2)) (|T:term| (_ BitVec 3)))
         (let ((a!1 (forall ((|N2:node| (_ BitVec 2)))
                      (=> (member |N2:node| (voting_quorum |T:term|))
                          (vote_msg |N1:node| |T:term| |N2:node|)))))
           (=> (leader |N1:node| |T:term|) a!1)))
       a!2
       a!3
       a!4
       a!5
       a!6
       a!7
       a!8
       a!9
       a!10
       a!11
       a!12
       a!13
       (forall ((|EI:index| (_ BitVec 3))
                (|ET:term| (_ BitVec 3))
                (|EV:value| (_ BitVec 2))
                (|PLI:option_index| (_ BitVec 4))
                (|PLT:option_term| (_ BitVec 4))
                (|T:term| (_ BitVec 3)))
         (let ((a!1 (and (__term_at (term_log |T:term|) |EI:index| |ET:term|)
                         (= |EV:value|
                            (__value_at (term_log |T:term|) |EI:index|)))))
           (=> (append_msg |T:term|
                           |PLI:option_index|
                           |PLT:option_term|
                           |EI:index|
                           |ET:term|
                           |EV:value|)
               a!1)))
       a!14
       (forall ((|T:term| (_ BitVec 3))) (__valid (term_log |T:term|)))
       (forall ((|N:node| (_ BitVec 2))) (__valid (__current_log |N:node|)))
       (__valid commit_log)
       a!15
       a!16
       a!17
       a!18
       a!19
       a!20
       a!21
       a!22
       a!23
       a!24
       (forall ((|N:node| (_ BitVec 2)) (|T:term| (_ BitVec 3)))
         (=> (leader |N:node| |T:term|) (started |T:term|)))
       a!25
       a!26
       a!27
       (forall ((|I:index| (_ BitVec 3)) (|T:term| (_ BitVec 3)))
         (=> (__term_at commit_log |I:index| |T:term|)
             (bvule |T:term| commit_term)))
       a!28
       a!29
       (=> (not (= commit_index #b000)) (committed commit_index commit_value))
       (forall ((|I:index| (_ BitVec 3)) (|V:value| (_ BitVec 2)))
         (=> (committed |I:index| |V:value|)
             (committed commit_index commit_value)))
       a!30
       a!31
       a!32
       (=> (__index_used commit_log commit_index)
           (__term_at commit_log commit_index commit_term))
       a!33
       a!34
       a!35
       (or __ts0_e __ts0_f)
       (or (not __ts0_e) (__current_term |__fml:n| |__loc:t|))
       (or (not __ts0_e)
           (append_msg |__loc:t2|
                       |__loc:opi|
                       |__loc:opt|
                       |__loc:ei|
                       |__loc:et|
                       |__loc:ev|))
       (or (not __ts0_e) (bvuge |__loc:t2| |__loc:t|))
       (or (not __ts0_e)
           (not __ts0__ts0_g)
           (not __ts0__ts0__ts0_e_a_a)
           (or __ts0_a __ts0_b))
       a!36
       (forall ((|V0:log| (_ BitVec 4)))
         (let ((a!1 (or (not __ts0_b) (not (not (__valid |V0:log|))))))
           (or (not __ts0_e) (not __ts0__ts0_g) (not __ts0__ts0__ts0_e_a_a) a!1)))
       (or (not __ts0_e)
           (not __ts0__ts0_g)
           __ts0__ts0__ts0_e_a_a
           (or __ts0_c __ts0_d))
       a!37
       a!38
       (or (not __ts0_e)
           (not __ts0__ts0_g)
           __ts0__ts0__ts0_e_a_a
           (or (not __ts0_c) (or __ts0_a __ts0_b)))
       (or (not __ts0_e) (not __ts0__ts0_g) __ts0__ts0__ts0_e_a_a a!39)
       a!40
       a!41
       (or (not __ts0_e)
           __ts0__ts0_g
           (not __ts0__ts0__ts0_f_a)
           (not __ts0__ts0__ts0__ts0_e_a)
           (or __ts0_a __ts0_b))
       a!42
       (forall ((|V0:log| (_ BitVec 4)))
         (let ((a!1 (or (not __ts0_b) (not (not (__valid |V0:log|))))))
           (or (not __ts0_e)
               __ts0__ts0_g
               (not __ts0__ts0__ts0_f_a)
               (not __ts0__ts0__ts0__ts0_e_a)
               a!1)))
       (or (not __ts0_e)
           __ts0__ts0_g
           (not __ts0__ts0__ts0_f_a)
           __ts0__ts0__ts0__ts0_e_a
           (or __ts0_c __ts0_d))
       a!43
       a!44
       (or (not __ts0_e)
           __ts0__ts0_g
           (not __ts0__ts0__ts0_f_a)
           __ts0__ts0__ts0__ts0_e_a
           (or (not __ts0_c) (or __ts0_a __ts0_b)))
       (or (not __ts0_e)
           __ts0__ts0_g
           (not __ts0__ts0__ts0_f_a)
           __ts0__ts0__ts0__ts0_e_a
           a!39)
       a!45
       a!46
       a!47
       (forall ((|X:term| (_ BitVec 3))) (bvule |X:term| |X:term|))
       a!48
       a!49
       (forall ((|X:term| (_ BitVec 3)) (|Y:term| (_ BitVec 3)))
         (or (bvule |X:term| |Y:term|) (bvule |Y:term| |X:term|)))
       (forall ((|X:term| (_ BitVec 3))) (bvule #b000 |X:term|))
       a!50
       (forall ((|X:term| (_ BitVec 3)) (|Y:term| (_ BitVec 3)))
         (= (bvugt |X:term| |Y:term|) (bvult |Y:term| |X:term|)))
       (forall ((|X:term| (_ BitVec 3)) (|Y:term| (_ BitVec 3)))
         (= (bvuge |X:term| |Y:term|) (bvule |Y:term| |X:term|)))
       (forall ((|X:index| (_ BitVec 3))) (bvule |X:index| |X:index|))
       a!51
       a!52
       (forall ((|X:index| (_ BitVec 3)) (|Y:index| (_ BitVec 3)))
         (or (bvule |X:index| |Y:index|) (bvule |Y:index| |X:index|)))
       (forall ((|X:index| (_ BitVec 3))) (bvule #b000 |X:index|))
       a!53
       (forall ((|X:index| (_ BitVec 3)) (|Y:index| (_ BitVec 3)))
         (= (bvugt |X:index| |Y:index|) (bvult |Y:index| |X:index|)))
       (forall ((|X:index| (_ BitVec 3)) (|Y:index| (_ BitVec 3)))
         (= (bvuge |X:index| |Y:index|) (bvule |Y:index| |X:index|)))
       a!54
       (forall ((|OI:option_index| (_ BitVec 4)))
         (= (is_none_index |OI:option_index|) (= |OI:option_index| none_index)))
       (forall ((|OT:option_term| (_ BitVec 4)))
         (= (is_none_term |OT:option_term|) (= |OT:option_term| none_term)))
       (forall ((|I:index| (_ BitVec 3)) (|T:term| (_ BitVec 3)))
         (let ((a!1 (exists ((|Q:quorum| (_ BitVec 4)))
                      (forall ((|N:node| (_ BitVec 2)))
                        (=> (member |N:node| |Q:quorum|)
                            (or (__append_reply_msg
                                  |N:node|
                                  |T:term|
                                  |I:index|
                                  |T:term|)
                                (not (__term_bigger |N:node| |T:term|))))))))
           (= (__choosable |I:index| |T:term|) a!1)))
       (= |__ts0__new_fml:n_a| |__fml:n|)
       (= |__ts0__new_fml:t| |__loc:t2|)
       (forall ((|V0:node| (_ BitVec 2)) (|V1:term| (_ BitVec 3)))
         (let ((a!1 (ite __ts0_f
                         (__current_term |V0:node| |V1:term|)
                         (ite (and (= |V0:node| |__ts0__new_fml:n_a|))
                              (= |V1:term| |__ts0__new_fml:t|)
                              (__current_term |V0:node| |V1:term|)))))
           (= (current_term |V0:node| |V1:term|) a!1)))
       (forall ((|V0:node| (_ BitVec 2)) (|V1:term| (_ BitVec 3)))
         (let ((a!1 (ite __ts0_f
                         (__term_bigger |V0:node| |V1:term|)
                         (ite (and (= |V0:node| |__ts0__new_fml:n_a|))
                              (bvult |V1:term| |__ts0__new_fml:t|)
                              (__term_bigger |V0:node| |V1:term|)))))
           (= (term_bigger |V0:node| |V1:term|) a!1)))
       a!55
       a!56
       a!57
       a!58
       a!59
       a!60
       a!61
       (= |__ts0__ts0__new_loc:pi_a| (oi_index |__loc:opi|))
       (= |__ts0__ts0__new_loc:pt_a| (ot_term |__loc:opt|))
       a!62
       (= __ts0__ts0__ts0_f_a
          (__term_at (__current_log |__fml:n|)
                     |__ts0__ts0__new_loc:pi_a|
                     |__ts0__ts0__new_loc:pt_a|))
       (= __ts0__ts0_g (is_none_index |__loc:opi|))
       (forall ((|I:index| (_ BitVec 3)) (|T:term| (_ BitVec 3)))
         (let ((a!1 (exists ((|Q:quorum| (_ BitVec 4)))
                      (forall ((|N:node| (_ BitVec 2)))
                        (=> (member |N:node| |Q:quorum|)
                            (or (__append_reply_msg
                                  |N:node|
                                  |T:term|
                                  |I:index|
                                  |T:term|)
                                (not (term_bigger |N:node| |T:term|))))))))
           (= (__ts0__m_choosable |I:index| |T:term|) a!1)))
       (forall ((|I:index| (_ BitVec 3)) (|T:term| (_ BitVec 3)))
         (let ((a!1 (exists ((|Q:quorum| (_ BitVec 4)))
                      (forall ((|N:node| (_ BitVec 2)))
                        (=> (member |N:node| |Q:quorum|)
                            (or (append_reply_msg |N:node|
                                                  |T:term|
                                                  |I:index|
                                                  |T:term|)
                                (not (term_bigger |N:node| |T:term|))))))))
           (= (choosable |I:index| |T:term|) a!1))))))
(assert (let ((a!1 (not (=> (and (choosable @I @T1) (append_reply_msg @N @T1 @I @T1))
                    (term_at (current_log @N) @I @T1)))))
  (and a!1)))

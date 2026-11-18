/*++
Copyright (c) 2026 Jason E. Aten, Ph.D. (insdict, MIT License)

Module Name:

    insdict.cpp

Abstract:

    Tests for util/insdict.h, ported from the Go package
    github.com/glycerine/insdict v0.14.6 (insdict_test.go, legacy_test.go),
    plus a structural-equality key case (the ast_table use) and golden
    capacity/order vectors produced by the Go insdict itself.

    FORK FIX #140 (owner decision 2026-11-18, see FORK_BUGFIXES.md).

--*/
#include <algorithm>
#include <iostream>
#include "util/insdict.h"
#include "util/hashtable.h"
#include "util/vector.h"
#include "util/util.h"

namespace {

struct mix_hash {
    uint64_t operator()(int k) const { return insdict_mix64(static_cast<uint64_t>(static_cast<int64_t>(k))); }
};

struct mod_hash {
    uint64_t m;
    mod_hash(uint64_t m = 3): m(m) {}
    uint64_t operator()(int k) const { return static_cast<uint64_t>(static_cast<int64_t>(k)) % m; }
};

struct const_hash {
    uint64_t operator()(int) const { return 0; }
};

struct int_eq {
    bool operator()(int a, int b) const { return a == b; }
};

// Exposes the internals for the structural invariant checks.
template<typename H>
class tdict : public insdict<int, H, int_eq> {
    typedef insdict<int, H, int_eq> base;
public:
    tdict(H const & h = H()): base(h) {}
    using base::m_indices;
    using base::m_tags;
    using base::m_size;
    using base::m_mask;
    using base::m_entries;
    using base::m_num_entries;
    using base::m_live;
    using base::usable;
    using base::size_for;
    using base::rebuild;
    using base::get_hash;
    uint64_t probe_find(int k) const {
        uint64_t h = this->get_hash(k);
        uint64_t slot;
        return static_cast<uint64_t>(static_cast<int64_t>(this->find_core(k, h, base::hash_tag(h), int_eq(), slot)));
    }
    unsigned char tag_of(uint64_t h) const { return base::hash_tag(h); }
};

// The reference model: presence plus an explicit insertion order.
struct model {
    int_hashtable<int_hash, default_eq<int> > m;
    svector<int> order;
    void put(int k) {
        if (!m.contains(k)) {
            m.insert(k);
            order.push_back(k);
        }
    }
    bool del(int k) {
        if (!m.contains(k))
            return false;
        m.erase(k);
        unsigned j = 0;
        for (unsigned i = 0; i < order.size(); i++)
            if (order[i] != k)
                order[j++] = order[i];
        order.shrink(j);
        return true;
    }
};

// A small deterministic generator (the golden vectors use the same one).
struct lcg {
    uint64_t x;
    lcg(uint64_t s): x(s) {}
    uint64_t next() {
        x = x * 6364136223846793005ull + 1442695040888963407ull;
        return x >> 33;
    }
};

bool same_seq(svector<int> const & a, svector<int> const & b) {
    if (a.size() != b.size())
        return false;
    for (unsigned i = 0; i < a.size(); i++)
        if (a[i] != b[i])
            return false;
    return true;
}

template<typename H>
void keys_of(tdict<H> const & d, svector<int> & out) {
    out.reset();
    for (int k : d)
        out.push_back(k);
}

template<typename H>
void check_invariants(tdict<H> const & d) {
    if (d.m_indices == nullptr) {
        ENSURE(d.m_live == 0 && d.m_num_entries == 0 && d.m_size == 0 && d.capacity() == 0);
        return;
    }
    unsigned size = d.m_size;
    ENSURE(d.capacity() == size);
    ENSURE(size >= 8 && (size & (size - 1)) == 0);
    ENSURE(d.m_mask == static_cast<uint64_t>(size - 1));
    ENSURE(d.m_num_entries <= d.usable());
    ENSURE(reinterpret_cast<unsigned char const *>(d.m_indices + size) == d.m_tags);
    unsigned live = 0;
    for (unsigned i = 0; i < d.m_num_entries; i++) {
        auto const & e = d.m_entries[i];
        if (!e.m_live) {
            ENSURE(e.m_hash == 0 && e.m_key == 0);
            continue;
        }
        live++;
        ENSURE(d.get_hash(e.m_key) == e.m_hash);
        ENSURE(d.probe_find(e.m_key) == i);
    }
    ENSURE(live == d.m_live);
    unsigned refs = 0, empties = 0;
    svector<unsigned> references;
    references.resize(d.m_num_entries, 0);
    for (unsigned slot = 0; slot < size; slot++) {
        int ix = d.m_indices[slot];
        if (ix == -1) {
            empties++;
            ENSURE(d.m_tags[slot] == 0);
        }
        else if (ix == -2) {
            ENSURE(d.m_tags[slot] == 1);
        }
        else {
            ENSURE(ix >= 0 && static_cast<unsigned>(ix) < d.m_num_entries && d.m_entries[ix].m_live);
            refs++;
            references[ix]++;
            uint64_t h = d.m_entries[ix].m_hash;
            uint64_t probe = h & d.m_mask, perturb = h;
            bool reachable = false;
            for (unsigned step = 0; step < size + 13; step++) {
                if (probe == slot) {
                    reachable = true;
                    break;
                }
                if (d.m_tags[probe] == 0)
                    break;
                perturb >>= 5;
                probe = (probe * 5 + perturb + 1) & d.m_mask;
            }
            ENSURE(reachable);
            ENSURE(d.m_tags[slot] == d.tag_of(h));
        }
    }
    for (unsigned i = 0; i < d.m_num_entries; i++)
        ENSURE(references[i] == (d.m_entries[i].m_live ? 1u : 0u));
    ENSURE(refs == d.m_live);
    ENSURE(empties > 0);
}

template<typename H>
void check_against(tdict<H> const & d, model const & m) {
    check_invariants(d);
    ENSURE(d.size() == m.order.size());
    for (int k : m.order)
        ENSURE(d.contains(k));
    svector<int> ks;
    keys_of(d, ks);
    ENSURE(same_seq(ks, m.order));
}

void tst_empty() {
    tdict<mix_hash> d;
    ENSURE(d.size() == 0 && d.empty() && d.capacity() == 0);
    ENSURE(!d.contains(1));
    int r;
    ENSURE(!d.find(1, r));
    ENSURE(!d.erase(1));
    ENSURE(d.begin() == d.end());
    check_invariants(d);
    d.insert(1);
    ENSURE(d.contains(1) && d.capacity() == 8);
    check_invariants(d);
}

void tst_overwrite_keeps_position() {
    tdict<mix_hash> d;
    d.insert(1); d.insert(2); d.insert(3);
    d.insert(1); d.insert(2);
    svector<int> ks;
    keys_of(d, ks);
    ENSURE(ks.size() == 3 && ks[0] == 1 && ks[1] == 2 && ks[2] == 3);
    ENSURE(d.size() == 3);
    check_invariants(d);
}

void tst_insertion_order() {
    tdict<mix_hash> d;
    svector<int> perm;
    for (int i = 0; i < 500; i++)
        perm.push_back(i);
    lcg r(1);
    for (unsigned i = perm.size(); i > 1; i--)
        std::swap(perm[i - 1], perm[r.next() % i]);
    for (int k : perm)
        d.insert(k);
    svector<int> ks;
    keys_of(d, ks);
    ENSURE(same_seq(ks, perm));
    check_invariants(d);
}

void tst_delete_and_reinsert() {
    tdict<mix_hash> d;
    d.insert(1); d.insert(2); d.insert(3);
    ENSURE(d.erase(2));
    ENSURE(!d.erase(2));
    ENSURE(!d.erase(99));
    ENSURE(!d.contains(2) && d.size() == 2);
    svector<int> ks;
    keys_of(d, ks);
    ENSURE(ks.size() == 2 && ks[0] == 1 && ks[1] == 3);
    check_invariants(d);
    d.erase(1);
    d.insert(1);
    keys_of(d, ks);
    ENSURE(ks.size() == 2 && ks[0] == 3 && ks[1] == 1);
    check_invariants(d);
}

void tst_delete_all_then_reuse() {
    tdict<mix_hash> d;
    for (int i = 0; i < 100; i++)
        d.insert(i);
    for (int i = 0; i < 100; i++)
        ENSURE(d.erase(i));
    ENSURE(d.size() == 0 && d.begin() == d.end());
    check_invariants(d);
    d.insert(7);
    ENSURE(d.contains(7) && d.size() == 1);
    check_invariants(d);
    d.finalize();
    ENSURE(d.size() == 0 && d.capacity() == 0);
    check_invariants(d);
}

void tst_growth() {
    tdict<mix_hash> d;
    model m;
    for (int i = 0; i < 20000; i++) {
        d.insert(i * 7919);
        m.order.push_back(i * 7919);
        if (i % 997 == 0) {
            check_invariants(d);
            svector<int> ks;
            keys_of(d, ks);
            ENSURE(same_seq(ks, m.order));
        }
    }
    for (int k : m.order)
        ENSURE(d.contains(k));
    check_invariants(d);
}

void tst_shrink_after_mass_delete() {
    tdict<mix_hash> d;
    model m;
    const int n = 1000;
    for (int i = 0; i < n; i++) {
        d.insert(i);
        m.put(i);
    }
    for (int i = 0; i < n; i++) {
        if (i % 100 != 0) {
            d.erase(i);
            m.del(i);
        }
    }
    unsigned before = d.capacity();
    d.pack(false);
    check_against(d, m);
    ENSURE(d.m_num_entries < static_cast<unsigned>(n));
    ENSURE(!(d.m_num_entries > 32 && d.m_live < d.m_num_entries / 4));
    ENSURE(d.capacity() == 32 && before == 2048);
}

void tst_order_across_compaction() {
    tdict<mix_hash> d;
    model m;
    for (int i = 0; i < 200; i++) {
        d.insert(i);
        m.put(i);
    }
    for (int i = 1; i < 200; i += 2) {
        d.erase(i);
        m.del(i);
    }
    check_against(d, m);
    for (int i = 1000; i < 1100; i++) {
        d.insert(i);
        m.put(i);
    }
    check_against(d, m);
}

void tst_churn() {
    tdict<mix_hash> d;
    model m;
    const int window = 10;
    for (int i = 0; i < 200000; i++) {
        d.insert(i);
        m.put(i);
        if (i >= window) {
            d.erase(i - window);
            m.del(i - window);
        }
        if (i % 20011 == 0)
            check_against(d, m);
    }
    check_against(d, m);
    ENSURE(d.size() == static_cast<unsigned>(window));
}

void tst_repeated_put_delete_same_key() {
    tdict<mix_hash> d;
    for (int i = 0; i < 50000; i++) {
        d.insert(5);
        ENSURE(d.erase(5));
    }
    check_invariants(d);
    ENSURE(d.size() == 0);
}

void tst_size_for() {
    for (unsigned live = 0; live < 5000; live++) {
        unsigned size = tdict<mix_hash>::size_for(live);
        ENSURE(size >= 8 && (size & (size - 1)) == 0);
        ENSURE(size >= live * 3);
        ENSURE(live <= size * 2 / 3);
        ENSURE(size == 8 || size / 2 < live * 3);
    }
}

void tst_metadata_layout() {
    unsigned sizes[] = { 8, 16, 64, 256 };
    for (unsigned size : sizes) {
        tdict<mix_hash> d;
        d.rebuild(size);
        ENSURE(d.capacity() == size);
        for (unsigned i = 0; i < size; i++)
            d.m_tags[i] = static_cast<unsigned char>(i);
        for (unsigned i = 0; i < size; i++) {
            ENSURE(d.m_indices[i] == -1);
            d.m_indices[i] = static_cast<int>(i);
        }
        for (unsigned i = 0; i < size; i++)
            ENSURE(d.m_tags[i] == static_cast<unsigned char>(i));
        d.rebuild(size);
        check_invariants(d);
    }
}

template<typename H>
void colliding(H const & h) {
    tdict<H> d(h);
    model m;
    for (int i = 0; i < 300; i++) {
        d.insert(i);
        m.put(i);
    }
    check_against(d, m);
    for (int i = 0; i < 300; i += 2) {
        d.erase(i);
        m.del(i);
    }
    check_against(d, m);
    for (int i = 0; i < 300; i += 4) {
        d.insert(i);
        m.put(i);
    }
    check_against(d, m);
}

void tst_colliding_hash() {
    colliding(mod_hash(3));
    colliding(const_hash());
}

void tst_mix64_injective_sample() {
    svector<uint64_t> hs;
    for (uint64_t i = 0; i < 100000; i++)
        hs.push_back(insdict_mix64(i));
    std::sort(hs.begin(), hs.end());
    for (unsigned i = 1; i < hs.size(); i++)
        ENSURE(hs[i - 1] != hs[i]);
    // Go: insdict.Mix64(0), Mix64(1).
    ENSURE(insdict_mix64(0) == 0xe220a8397b1dcdafull);
    ENSURE(insdict_mix64(1) == 0x910a2dec89025cc1ull);
}

template<typename H>
void random_ops(H const & h, uint64_t seed, unsigned key_space, unsigned ops) {
    lcg r(seed);
    tdict<H> d(h);
    model m;
    for (unsigned i = 0; i < ops; i++) {
        int k = static_cast<int>(r.next() % key_space);
        switch (r.next() % 10) {
        case 0: case 1: case 2: case 3: case 4:
            d.insert(k);
            m.put(k);
            break;
        case 5: case 6: case 7:
            ENSURE(d.erase(k) == m.del(k));
            break;
        default:
            ENSURE(d.contains(k) == m.m.contains(k));
            break;
        }
        if (i % 101 == 0)
            check_against(d, m);
    }
    check_against(d, m);
}

void tst_property_random_ops() {
    for (uint64_t seed = 0; seed < 50; seed++) {
        lcg r(seed + 1000);
        random_ops(mix_hash(), seed, 1 + static_cast<unsigned>(r.next() % 500), 3000);
    }
    for (uint64_t seed = 0; seed < 20; seed++) {
        lcg r(seed + 2000);
        random_ops(mod_hash(1 + r.next() % 8), seed, 150, 2000);
    }
}

void tst_deterministic_layout() {
    tdict<mix_hash> a, b;
    for (int pass = 0; pass < 2; pass++) {
        tdict<mix_hash> & d = pass == 0 ? a : b;
        lcg r(99);
        for (int i = 0; i < 5000; i++) {
            int k = static_cast<int>(r.next() % 400);
            if (r.next() % 3 == 0)
                d.erase(k);
            else
                d.insert(k);
        }
    }
    ENSURE(a.m_size == b.m_size && a.m_num_entries == b.m_num_entries);
    ENSURE(memcmp(a.m_indices, b.m_indices, sizeof(int) * a.m_size) == 0);
    ENSURE(memcmp(a.m_tags, b.m_tags, a.m_size) == 0);
    for (unsigned i = 0; i < a.m_num_entries; i++)
        ENSURE(a.m_entries[i].m_key == b.m_entries[i].m_key && a.m_entries[i].m_live == b.m_entries[i].m_live);
}

void tst_swap() {
    tdict<mix_hash> a, b;
    for (int i = 0; i < 40; i++)
        a.insert(i);
    b.insert(-1);
    unsigned ca = a.capacity(), cb = b.capacity();
    a.swap(b);
    ENSURE(a.size() == 1 && b.size() == 40 && a.capacity() == cb && b.capacity() == ca);
    ENSURE(a.contains(-1) && b.contains(39) && !a.contains(0));
    check_invariants(a);
    check_invariants(b);
}

// Golden vectors from the Go insdict v0.14.6: the same lcg-driven
// Put/Del/Pack(false) stream; a FNV-1a fold of (len(indices),
// len(entries), live) after every operation and of the final key order,
// plus the first index-length changes. They depend only on the operation
// stream, not on the hash function.
struct golden_change { unsigned op; unsigned cap; };

uint64_t fnv(uint64_t h, uint64_t v) {
    for (int i = 0; i < 8; i++) {
        h ^= (v >> (8 * i)) & 0xff;
        h *= 1099511628211ull;
    }
    return h;
}

void golden(uint64_t seed, unsigned key_space, unsigned ops, unsigned cap, unsigned live, unsigned entries,
            uint64_t hash, golden_change const * changes, unsigned num_changes) {
    tdict<mix_hash> d;
    lcg r(seed);
    uint64_t h = 14695981039346656037ull;
    unsigned prev = 0, seen = 0;
    for (unsigned i = 0; i < ops; i++) {
        int k = static_cast<int>(r.next() % key_space);
        if (r.next() % 10 < 6)
            d.insert(k);
        else
            d.erase(k);
        if (r.next() % 500 == 0)
            d.pack(false);
        h = fnv(h, d.capacity());
        h = fnv(h, d.num_entries());
        h = fnv(h, d.size());
        if (d.capacity() != prev) {
            prev = d.capacity();
            if (seen < num_changes) {
                ENSURE(changes[seen].op == i && changes[seen].cap == prev);
                seen++;
            }
        }
    }
    for (int k : d)
        h = fnv(h, static_cast<uint64_t>(static_cast<int64_t>(k)));
    ENSURE(seen == num_changes);
    ENSURE(d.capacity() == cap && d.size() == live && d.num_entries() == entries);
    ENSURE(h == hash);
    check_invariants(d);
}

void tst_golden() {
    golden_change c1[] = { {0,8}, {5,16}, {14,32}, {47,64}, {109,128} };
    golden(1, 50, 4000, 128, 34, 63, 0x8179c5baef7d449cull, c1, 5);
    golden_change c2[] = { {0,8}, {6,16}, {13,32}, {39,64}, {66,128}, {149,256}, {298,512}, {617,1024},
                           {1338,2048}, {2989,4096}, {8147,8192} };
    golden(2, 3000, 20000, 8192, 1771, 4575, 0x739290284483937ull, c2, 11);
    golden_change c3[] = { {0,8}, {8,16}, {13,32}, {29,64}, {69,128}, {152,256}, {281,512}, {567,1024},
                           {1152,2048}, {2314,4096}, {4618,8192}, {9368,16384} };
    golden(3, 100000, 30000, 32768, 15377, 15900, 0x5e97b9897cc41b48ull, c3, 12);
}

// Structural-equality keys, as ast_table uses insdict: a probe that is a
// different object with equal contents finds the stored one.
struct node {
    int a, b;
    node(int a, int b): a(a), b(b) {}
};

struct node_hash {
    uint64_t operator()(node const * n) const {
        return insdict_mix64((static_cast<uint64_t>(static_cast<unsigned>(n->a)) << 32) | static_cast<unsigned>(n->b));
    }
};

struct node_eq {
    bool operator()(node const * x, node const * y) const { return x->a == y->a && x->b == y->b; }
};

void tst_structural_keys() {
    insdict<node *, node_hash, node_eq> d;
    svector<node *> owned;
    for (int i = 0; i < 100; i++) {
        node * n = alloc(node, i, i * 3);
        owned.push_back(n);
        ENSURE(d.insert_if_not_there(n) == n);
    }
    ENSURE(d.size() == 100);
    for (int i = 0; i < 100; i++) {
        node probe(i, i * 3);
        node * r = nullptr;
        ENSURE(d.find(&probe, r) && r == owned[i]);
        ENSURE(d.contains(&probe));
        // An equal object is not inserted: the stored one is returned.
        ENSURE(d.insert_if_not_there(&probe) == owned[i]);
        node other(i, i * 3 + 1);
        ENSURE(!d.contains(&other));
    }
    ENSURE(d.size() == 100);
    // Identity erase never matches an equal but distinct object.
    node probe(5, 15);
    ENSURE(!d.erase_identical(&probe));
    ENSURE(d.contains(&probe));
    ENSURE(d.erase_identical(owned[5]));
    ENSURE(!d.contains(&probe));
    // Structural erase removes the stored equal object.
    node probe7(7, 21);
    ENSURE(d.erase(&probe7));
    ENSURE(!d.contains(owned[7]));
    unsigned i = 0;
    for (node * n : d) {
        while (i == 5 || i == 7)
            i++;
        ENSURE(n == owned[i]);
        i++;
    }
    ENSURE(i == 100);
    for (node * n : owned)
        dealloc(n);
}

}

void tst_insdict() {
    tst_empty();
    tst_overwrite_keeps_position();
    tst_insertion_order();
    tst_delete_and_reinsert();
    tst_delete_all_then_reuse();
    tst_growth();
    tst_shrink_after_mass_delete();
    tst_order_across_compaction();
    tst_churn();
    tst_repeated_put_delete_same_key();
    tst_size_for();
    tst_metadata_layout();
    tst_colliding_hash();
    tst_mix64_injective_sample();
    tst_property_random_ops();
    tst_deterministic_layout();
    tst_swap();
    tst_golden();
    tst_structural_keys();
    std::cout << "insdict: ok\n";
}

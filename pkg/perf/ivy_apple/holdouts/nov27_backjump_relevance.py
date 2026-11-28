"""Break down backjump relevance samples from the seven-check study. Write CSV to stdout."""
import collections
import csv
import io
import sys
import tarfile
from pathlib import Path


def keys(row):
    frames = row["stack"].splitlines()
    if row["engine"] == "g3":
        conflict = "SMTKernel).resolveSearchConflict"
        pop = "SMTRelevancyPropagator).PopScopes"
    else:
        conflict = "smt::context::resolve_conflict("
        pop = "smt::relevancy_propagator_imp::pop("
    outer = next((i for i, frame in enumerate(frames) if conflict in frame), 0)
    inner = next((i for i, frame in enumerate(frames[:outer]) if pop in frame), None)
    if inner is None:
        return
    path = "\n".join(frames[:inner])
    operation = "pop or inlined work"
    if "SMTRelevancyPropagator).undoTrail" in path:
        operation = "watch and handler undo"
    elif "ShrinkHeldRefs" in path:
        operation = "release marked expressions"
    elif "smtRelevantTable.del" in path:
        operation = "delete relevance mark"
    deletion = "deleteNodeWorklistBody" in path or "ast_manager::delete_node(" in path
    yield row["case"], row["batch"], row["goal"], row["engine"], operation, int(deletion)


def read_samples():
    if len(sys.argv) == 2:
        with open(sys.argv[1]) as source:
            yield from csv.DictReader(source)
        return
    archive = Path(__file__).with_name("nov27-arm-ticket-check-costs.tar.xz")
    with tarfile.open(archive) as study:
        with io.TextIOWrapper(study.extractfile("study/stacks.csv")) as source:
            yield from csv.DictReader(source)


def main():
    counts = collections.Counter()
    periods = collections.Counter()
    rounds = collections.defaultdict(set)
    for row in read_samples():
        rounds[row["case"], row["batch"], row["goal"], row["engine"]].add(row["round"])
        for key in keys(row):
            samples = int(row["samples"])
            counts[key] += samples
            periods[key] += samples * int(row["period"])
    writer = csv.writer(sys.stdout)
    writer.writerow(["case", "batch", "goal", "engine", "operation", "in_ast_deletion",
                     "samples_across_runs", "mean_sampled_instructions"])
    for key in sorted(counts):
        writer.writerow([*key, counts[key], round(periods[key] / len(rounds[key[:4]]))])


if __name__ == "__main__":
    main()

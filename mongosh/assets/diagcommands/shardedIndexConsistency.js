try {
  printjson(db.serverStatus().shardedIndexConsistency);
} catch (e) {
  print("ERROR: " + e);
}

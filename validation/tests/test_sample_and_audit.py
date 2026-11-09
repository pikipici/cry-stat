"""Deterministic, network-free P2 regression tests."""
from __future__ import annotations
import csv, json, pathlib, sys, tempfile, unittest
from unittest import mock
ROOT=pathlib.Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/"validation/bin"))
import sample_and_audit as audit  # noqa: E402
MINT="So11111111111111111111111111111111111111112"
PAIR="9wFFmGph15h76UzGCrErN5nG3EqUr6xR3LLuW2hQ2Vj9"
QUOTE="Es9vMFrzaCERmJfrF4H2FYD5LKYsx6PjnQZpQCr7GGT"

def sample(i=0,stratum="active_liquid"):
 return {"mint":MINT,"stratum":stratum,"source":"fixture:list-v1","observed_at_utc":"2026-01-01T00:00:00Z","age_seconds":10,"expected_supply_raw":"1000","reviewer":"fixture-reviewer","rpc":{"account_info":{"owner":"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA","data":{"parsed":{"info":{"mintAuthority":None,"freezeAuthority":None}}}},"supply":{"amount":"1000","decimals":9,"slot":123},"largest_accounts":[{"address":PAIR,"amount":"10"}]},"dex":{"pair":{"chainId":"solana","pairAddress":PAIR,"baseToken":{"address":MINT},"quoteToken":{"address":QUOTE},"priceUsd":"1.25"}}}

class Semantics(unittest.TestCase):
 def test_supply_falls_back_to_parsed_mint_raw_supply(self):
  s=sample(); del s["expected_supply_raw"]
  s["rpc"]["account_info"]["data"]["parsed"]["info"]["supply"]="1000"
  self.assertEqual(audit.normalize(s)["supply_match"],"TRUE")
  s["rpc"]["supply"]["amount"]="1001"
  self.assertEqual(audit.normalize(s)["supply_match"],"FALSE")
 def test_freshness_derived_from_timestamps_and_invalid_is_unknown(self):
  s=sample(); del s["age_seconds"]; s["received_at_utc"]="2026-01-01T00:04:59Z"
  self.assertEqual(audit.normalize(s)["freshness_ok"],"TRUE")
  s["received_at_utc"]="2026-01-01T00:05:01Z"
  self.assertEqual(audit.normalize(s)["freshness_ok"],"FALSE")
  s["received_at_utc"]="not-a-time"
  self.assertEqual(audit.normalize(s)["freshness_ok"],"UNKNOWN")
 def test_exact_base_mint_and_price_pass(self):
  r=audit.normalize(sample()); self.assertEqual((r["mint_match"],r["price_basis_match"],r["holder_semantics_valid"]),("TRUE","TRUE","TRUE"))
 def test_target_in_quote_rejects_price(self):
  s=sample(); s["dex"]["pair"]["baseToken"]["address"]=QUOTE; s["dex"]["pair"]["quoteToken"]["address"]=MINT
  r=audit.normalize(s); self.assertEqual(r["mint_match"],"FALSE"); self.assertEqual(r["price_basis_match"],"FALSE"); self.assertIn("TARGET_NOT_DEX_BASE",r["discrepancy_code"])
 def test_missing_critical_is_unknown_not_safe_or_zero(self):
  s=sample(); s["rpc"]["account_info"]={}; s["dex"]={}; s["rpc"]["largest_accounts"]=None
  r=audit.normalize(s); self.assertEqual(r["mint_match"],"UNKNOWN"); self.assertEqual(r["holder_semantics_valid"],"UNKNOWN"); self.assertIn("DATA_TIDAK_CUKUP",r["discrepancy_code"])
 def test_missing_authority_keys_are_unknown_but_explicit_null_is_observed(self):
  self.assertNotIn("AUTHORITY_DATA_MISSING",audit.normalize(sample())["discrepancy_code"])
  s=sample(); s["rpc"]["account_info"]["data"]={}
  r=audit.normalize(s); self.assertIn("AUTHORITY_DATA_MISSING",r["discrepancy_code"]); self.assertIn("DATA_TIDAK_CUKUP",r["discrepancy_code"])
 def test_largest_are_token_accounts_not_owners(self):
  s=sample(); s["holders_are_unique_owners"]=True
  r=audit.normalize(s); self.assertEqual(r["holder_semantics_valid"],"FALSE"); self.assertIn("HOLDER_SEMANTICS_INVALID",r["discrepancy_code"])
 def test_hash_is_deterministic_and_sensitive_to_raw(self):
  a=audit.normalize(sample()); b=audit.normalize(sample()); self.assertEqual(a["raw_sha256"],b["raw_sha256"]); s=sample(); s["age_seconds"]=11; self.assertNotEqual(a["raw_sha256"],audit.normalize(s)["raw_sha256"])

class SamplingAndArtifacts(unittest.TestCase):
 def setUp(self): self.samples=[sample(i,st) for st in audit.STRATA for i in range(25)]
 def test_failed_samples_remain_in_denominator(self):
  self.samples[0]["collection_error"]="TimeoutError"; self.samples[0]["rpc"]={}; self.samples[0]["dex"]={}
  q=audit.validate_sampling(self.samples); self.assertEqual(q["total"],100); self.assertEqual(q["failed_in_denominator"],1); self.assertTrue(q["strata_minimum_25"])
  rows=[audit.normalize(x) for x in self.samples]; self.assertEqual(len(rows),100); self.assertIn("COLLECTION_FAILED",rows[0]["discrepancy_code"])
 def test_exact_csv_contract_and_report_structure(self):
  rows=[audit.normalize(x) for x in self.samples]
  with tempfile.TemporaryDirectory() as d:
   d=pathlib.Path(d); audit.write_csv(d/"audit_100_tokens.csv",rows); audit.write_report(d/"DATA_ACCURACY.md","fixed-run",audit.validate_sampling(self.samples),rows,True)
   with (d/"audit_100_tokens.csv").open(newline="",encoding="utf-8") as f: got=list(csv.DictReader(f))
   self.assertEqual(tuple(got[0]),audit.COLUMNS); self.assertEqual(len(got),100)
   text=(d/"DATA_ACCURACY.md").read_text();
   for heading in ("## Run provenance","## Sampling frame","## Accuracy summary","## Confusion / error taxonomy","## Ten manual audit examples","## Interpretation constraints"): self.assertIn(heading,text)
 def test_stratum_shortfall_fails_gate_without_fabrication(self):
  q=audit.validate_sampling(self.samples[:-1]); self.assertEqual(q["total"],99); self.assertFalse(q["minimum_100"]); self.assertFalse(q["strata_minimum_25"])
 def test_acceptance_verdict_uses_explicit_p2_gates(self):
  rows=[audit.normalize(x) for x in self.samples]
  sampling=audit.validate_sampling(self.samples); sampling["distinct_valid_mints"]=100
  verdict=audit.evaluate_acceptance(sampling,rows)
  self.assertEqual(verdict["verdict"],"PASS"); self.assertEqual(verdict["verifiable_match_rate"],1.0)
  for row in rows[:11]: row["supply_match"]="FALSE"
  self.assertEqual(audit.evaluate_acceptance(sampling,rows)["verdict"],"FAIL")
 def test_documented_no_pair_is_unknown_not_silent_success(self):
  s=sample(stratum="risky_delisted_migrated"); s["dex"]={}; s["selection_rationale"]="documented mint; no Solana DEX pair observed at sampling time"
  row=audit.normalize(s); self.assertEqual((row["mint_match"],row["price_basis_match"]),("UNKNOWN","UNKNOWN"))
  verdict=audit.evaluate_acceptance(audit.validate_sampling([s]),[row])
  self.assertEqual(verdict["documented_unverifiable_no_pair"],1); self.assertEqual(verdict["silent_success_no_pair"],0)

class SamplingFrameBuilder(unittest.TestCase):
 def test_gecko_trending_parser_uses_included_base_token_defensively(self):
  def mint(n): return audit._b58encode(n.to_bytes(32,"big"))
  payload={"data":[
   {"relationships":{"base_token":{"data":{"type":"token","id":"solana_"+mint(1)}}}},
   {"relationships":{"base_token":{"data":{"type":"token","id":"resource-2"}}}},
   {"relationships":{"base_token":{"data":None}}}, {"relationships":None}],
   "included":[
    {"type":"token","id":"solana_"+mint(1),"attributes":{"address":mint(1)}},
    {"type":"token","id":"resource-2","attributes":{"address":mint(2)}},
    {"type":"token","id":"unused","attributes":{"address":mint(3)}}]}
  got=audit.parse_gecko_base_tokens(payload,"https://gecko/page=1")
  self.assertEqual([x["address"] for x in got],[mint(1),mint(2)])
  self.assertTrue(all(x["_candidate_source_urls"]==["https://gecko/page=1"] for x in got))
 def test_gecko_candidate_requires_exact_base_and_active_thresholds(self):
  def mint(n): return audit._b58encode(n.to_bytes(32,"big"))
  gecko=[{"address":mint(i),"_candidate_sources":["GeckoTerminal Solana trending pools"],"_candidate_source_urls":[f"https://g?page={i}"]} for i in range(1,5)]
  pairs={
   mint(1):[{"chainId":"solana","baseToken":{"address":mint(1)},"liquidity":{"usd":100000},"txns":{"h24":{"buys":50,"sells":50}}}],
   mint(2):[{"chainId":"solana","baseToken":{"address":mint(2)},"liquidity":{"usd":99999},"txns":{"h24":{"buys":5}}}],
   mint(3):[{"chainId":"solana","baseToken":{"address":mint(3)},"liquidity":{"usd":200000},"txns":{"h24":{"buys":99}}}],
   mint(4):[{"chainId":"solana","baseToken":{"address":mint(99)},"liquidity":{"usd":200000},"txns":{"h24":{"buys":200}}}]}
  frame=audit.build_sampling_frame([],gecko,lambda ms:{m:pairs.get(m,[]) for m in ms},"now")
  self.assertEqual([(x["mint"],x["stratum"]) for x in frame],[(mint(1),"active_liquid"),(mint(2),"low_liquidity"),(mint(4),"risky_delisted_migrated")])
  self.assertNotIn(mint(3),[x["mint"] for x in frame])
  active=frame[0]; self.assertIn("GeckoTerminal",active["source"]); self.assertEqual(active["provenance"]["candidate_source_urls"],["https://g?page=1"])
 def test_candidate_selection_prioritizes_documented_identity_then_spans_list(self):
  def mint(n): return audit._b58encode(n.to_bytes(32,"big"))
  tokens=[{"address":mint(i),"extensions":{"coingeckoId":f"coin-{i:02d}"}} if i in (3,7,12) else {"address":mint(i),"tags":["old-registry"]} for i in range(1,20)]
  got=audit.select_documented_candidates(tokens,5)
  self.assertEqual([x["extensions"]["coingeckoId"] for x in got[:3]],["coin-03","coin-07","coin-12"])
  self.assertEqual(len({x["address"] for x in got}),5)
 def test_three_discovery_feeds_are_combined_and_provenance_records_each_url(self):
  def mint(n): return audit._b58encode(n.to_bytes(32,"big"))
  feeds={"profiles":[{"chainId":"solana","tokenAddress":mint(1)}],"latest_boosts":[{"chainId":"solana","tokenAddress":mint(2)}],"top_boosts":[{"chainId":"solana","tokenAddress":mint(3)}]}
  urls={"profiles_url":"https://p","latest_boosts_url":"https://l","top_boosts_url":"https://t","tokens_url":"https://j"}
  frame=audit.build_sampling_frame(feeds,[],lambda ms:{m:[] for m in ms},"now",urls)
  self.assertEqual({x["mint"] for x in frame},{mint(1),mint(2),mint(3)})
  self.assertTrue(all("launch age not assumed" in x["selection_rationale"] for x in frame))
  self.assertTrue(all(x["provenance"]["discovery_feed_urls"]==["https://p","https://l","https://t"] for x in frame))
 def test_observable_rules_produce_distinct_balanced_frame_with_provenance(self):
  def mint(n): return audit._b58encode(n.to_bytes(32,"big"))
  profiles=[{"chainId":"solana","tokenAddress":mint(i),"url":"https://dexscreener.com/solana/x"} for i in range(1,31)]
  documented=[{"address":mint(i),"name":f"documented-{i}"} for i in range(31,151)]
  pairs={}
  for i in range(1,31): pairs[mint(i)]=[{"chainId":"solana","pairAddress":mint(200+i),"baseToken":{"address":mint(i)},"quoteToken":{"address":MINT},"liquidity":{"usd":1},"txns":{"h24":{"buys":0,"sells":0}}}]
  for i in range(31,56): pairs[mint(i)]=[{"chainId":"solana","pairAddress":mint(200+i),"baseToken":{"address":mint(i)},"quoteToken":{"address":MINT},"liquidity":{"usd":200000},"txns":{"h24":{"buys":80,"sells":80}}}]
  for i in range(56,81): pairs[mint(i)]=[{"chainId":"solana","pairAddress":mint(200+i),"baseToken":{"address":mint(i)},"quoteToken":{"address":MINT},"liquidity":{"usd":1000},"txns":{"h24":{"buys":2,"sells":1}}}]
  frame=audit.build_sampling_frame(profiles,documented,lambda addresses:{a:pairs.get(a,[]) for a in addresses},"2026-01-01T00:00:00Z")
  self.assertEqual(audit.validate_sampling(frame)["counts"],{s:25 for s in audit.STRATA})
  self.assertEqual(len({x["mint"] for x in frame}),100)
  for row in frame:
   self.assertTrue(row["source"]); self.assertTrue(row["selection_rationale"]); self.assertTrue(row["provenance"])
  risky=[x for x in frame if x["stratum"]=="risky_delisted_migrated"]
  self.assertTrue(all(x["selection_rationale"]=="documented mint; no Solana DEX pair observed at sampling time" for x in risky))
 def test_shortfall_is_reported_not_filled_by_duplicates(self):
  self.assertEqual(audit.build_sampling_frame([],[],lambda _: {},"2026-01-01T00:00:00Z"),[])

class LiveSafety(unittest.TestCase):
 def test_local_env_loads_without_overriding_process_environment(self):
  with tempfile.TemporaryDirectory() as d:
   env=pathlib.Path(d)/".env"; env.write_text("SOLANA_RPC_URL=https://dedicated.example/?api-key=secret\nOTHER=value\n",encoding="utf-8")
   with mock.patch.dict(audit.os.environ,{"SOLANA_RPC_URL":"https://already-set.example"},clear=True):
    audit.load_local_env(env)
    self.assertEqual(audit.os.environ["SOLANA_RPC_URL"],"https://already-set.example")
    self.assertEqual(audit.os.environ["OTHER"],"value")
 def test_only_read_rpc_methods_exist(self):
  source=pathlib.Path(audit.__file__).read_text(encoding="utf-8")
  for forbidden in ("sendTransaction","signTransaction","simulateTransaction","getKeypair"): self.assertNotIn(forbidden,source)
 def test_http_rpc_rejected_before_io(self):
  with self.assertRaises(ValueError): audit._request("http://rpc.invalid",{},0.01)
 def test_browser_user_agent_and_retries(self):
  response=mock.MagicMock(); response.__enter__.return_value=response; response.read.return_value=b"{}"
  err=audit.urllib.error.HTTPError("https://x",429,"rate",{},None)
  with mock.patch.object(audit.urllib.request,"urlopen",side_effect=[err,response]) as opened, mock.patch.object(audit.time,"sleep") as slept:
   self.assertEqual(audit._request("https://x",None,1,retries=1),{})
   self.assertIn("Mozilla/5.0",opened.call_args_list[-1].args[0].headers["User-agent"]); slept.assert_called_once()
 def test_largest_accounts_429_preserves_account_supply_and_dex(self):
  account={"result":{"value":{"owner":"token-program"}}}; supply={"result":{"context":{"slot":77},"value":{"amount":"12"}}}
  rate=audit.urllib.error.HTTPError("https://rpc.example/?api-key=SECRET",429,"rate",{},None)
  pairs=[{"chainId":"solana","baseToken":{"address":MINT},"liquidity":{"usd":1}}]
  with mock.patch.object(audit,"_request",side_effect=[account,supply,rate,pairs]), mock.patch.object(audit.time,"sleep"):
   got=audit.collect_live({"mint":MINT},"https://rpc.example/?api-key=SECRET",1,0)
  self.assertEqual(got["rpc"]["account_info"],account["result"]["value"]); self.assertEqual(got["rpc"]["supply"]["amount"],"12")
  self.assertIsNone(got["rpc"]["largest_accounts"]); self.assertTrue(got["dex"]["pair"]); self.assertNotIn("collection_error",got)
  failure=got["collection_status"]["getTokenLargestAccounts"]
  self.assertEqual((failure["status"],failure["error"]["type"],failure["error"]["http_status"]),("error","HTTPError",429)); self.assertNotIn("SECRET",json.dumps(got))
 def test_every_request_is_independent_and_total_failure_sets_top_level_error(self):
  errors=[TimeoutError("secret one"),ValueError("secret two"),OSError("secret three"),RuntimeError("secret four")]
  with mock.patch.object(audit,"_request",side_effect=errors), mock.patch.object(audit.time,"sleep"):
   got=audit.collect_live({"mint":MINT},"https://rpc.example",1,0)
  self.assertEqual(set(got["collection_status"]),{"getAccountInfo","getTokenSupply","getTokenLargestAccounts","dex_pairs"})
  self.assertTrue(all(x["status"]=="error" for x in got["collection_status"].values())); self.assertEqual(got["collection_error"],"all_requests_failed"); self.assertNotIn("secret",json.dumps(got).lower())
 def test_json_rpc_error_is_structured_and_does_not_stop_later_methods(self):
  responses=[{"error":{"code":-32000,"message":"sensitive provider detail"}}, {"result":{"context":{"slot":2},"value":{"amount":"3"}}}, {"result":{"value":[]}}, []]
  with mock.patch.object(audit,"_request",side_effect=responses), mock.patch.object(audit.time,"sleep"):
   got=audit.collect_live({"mint":MINT},"https://rpc.example",1,0)
  self.assertEqual(got["collection_status"]["getAccountInfo"]["error"],{"type":"JSONRPCError","code":-32000}); self.assertEqual(got["rpc"]["supply"]["amount"],"3")
  self.assertNotIn("sensitive",json.dumps(got)); self.assertNotIn("collection_error",got)

class Checkpoint(unittest.TestCase):
 def test_resume_preserves_completed_failures(self):
  with tempfile.TemporaryDirectory() as d:
   path=pathlib.Path(d)/"checkpoint.jsonl"; items=[{"mint":MINT},{"mint":PAIR}]; calls=[]
   def collect(x,*_): calls.append(x["mint"]); return dict(x,collection_error="TimeoutError")
   first=audit.collect_with_checkpoint(items,path,collect,"https://rpc",1,0)
   second=audit.collect_with_checkpoint(items,path,collect,"https://rpc",1,0)
   self.assertEqual((len(first),len(second)),(2,2)); self.assertEqual(calls,[MINT,PAIR])
   self.assertTrue(all("collection_error" in x for x in second))
 def test_new_checkpoint_retries_without_mutating_failed_checkpoint(self):
  with tempfile.TemporaryDirectory() as d:
   old=pathlib.Path(d)/"failed.jsonl"; new=pathlib.Path(d)/"retry.jsonl"
   old.write_text(json.dumps({"mint":MINT,"collection_error":"all_requests_failed"})+"\n"); before=old.read_bytes(); calls=[]
   def collect(x,*_): calls.append(x["mint"]); return dict(x,rpc={"supply":{"amount":"1"}})
   got=audit.collect_with_checkpoint([{"mint":MINT}],new,collect,"https://rpc",1,0)
   self.assertEqual(calls,[MINT]); self.assertEqual(old.read_bytes(),before); self.assertNotIn("collection_error",got[0])

class FrameCLI(unittest.TestCase):
 def test_shortfall_artifact_is_written_without_frame_fabrication(self):
  with tempfile.TemporaryDirectory() as d, mock.patch.object(audit,"ROOT",pathlib.Path(d)), mock.patch.object(audit,"fetch_sampling_sources",return_value=([],[],{"profiles_url":"https://example/profiles","tokens_url":"https://example/tokens"})):
   rc=audit.main(["--build-frame","--frame-output","frame.json","--shortfall-output","shortfall.json"])
   self.assertEqual(rc,2); self.assertFalse((pathlib.Path(d)/"frame.json").exists())
   status=json.loads((pathlib.Path(d)/"shortfall.json").read_text()); self.assertEqual(status["status"],"SHORTFALL"); self.assertEqual(status["sampling"]["distinct_valid_mints"],0)

if __name__=="__main__": unittest.main()

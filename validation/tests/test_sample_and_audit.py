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

class SamplingFrameBuilder(unittest.TestCase):
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

class FrameCLI(unittest.TestCase):
 def test_shortfall_artifact_is_written_without_frame_fabrication(self):
  with tempfile.TemporaryDirectory() as d, mock.patch.object(audit,"ROOT",pathlib.Path(d)), mock.patch.object(audit,"fetch_sampling_sources",return_value=([],[],{"profiles_url":"https://example/profiles","tokens_url":"https://example/tokens"})):
   rc=audit.main(["--build-frame","--frame-output","frame.json","--shortfall-output","shortfall.json"])
   self.assertEqual(rc,2); self.assertFalse((pathlib.Path(d)/"frame.json").exists())
   status=json.loads((pathlib.Path(d)/"shortfall.json").read_text()); self.assertEqual(status["status"],"SHORTFALL"); self.assertEqual(status["sampling"]["distinct_valid_mints"],0)

if __name__=="__main__": unittest.main()

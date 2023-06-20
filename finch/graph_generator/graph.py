from enum import Enum
import requests
from web3 import Web3
from decimal import Decimal, getcontext
from graph_tool.all import *
import json
import os

# ------------- Constants for querying thegraph
PAGINATION = 3
FIRST = 1000

blacklist_pairs = [
  '0xeBD49b4c8f7F0deD2cA8B951Cf92a583e7b4C8e7',
  '0x5a8bd8cdd4dA5aEbB783D2c67125bE0df484eEFe',
  '0x32a505BF9dB617d23bF3EbaAc9aeF80cB24a828C',
  '0x149148aCC3b06b8Cc73Af3A10E84189243A35925',
  '0xbE44E5dd67276F53b2Aca540B4eD32c84eD0e7B3',
  '0x2059024d050cdAfe4e4850596Ff1490cFc40c7Bd',
  '0xadeAa96A81eBBa4e3A5525A008Ee107385d588C3',
  '0xD3FF34FE9692c563bAfCb58c9A085C8C89c2F180',
  '0x17202e8E31FE0eeBdb401B2e11492F5Be52c8312',
  '0x6D7a7b251a6cAC43F1798494d847e6D333f9FFCa',
  '0x3150e4162DfDD5c89A8653bDf1CbB9E09C11F42a',
  '0x290CCAA8CC8e21d16f042d9AE0De9aC1805B824f',
  '0x20dd5Acc97E3f8685A9da6E499B21B22A7967279',
  '0xbaCd80B88104586D1CC9bcEa999781F3d393C332'
]

# ------------- graph-tool and edge properties setup
# This is imported into variant_utils.py to create eprops for each variant
g = Graph(directed=False)

# Fixed props
token0_prop = g.new_edge_property("string")
token1_prop = g.new_edge_property("string")
pair_prop = g.new_edge_property("string")
fixed_eprops = [token0_prop, token1_prop, pair_prop]

# UniswapV2 props
uniswapv2_info = ["reserves0", "reserves1"]
uniswapv2_eprops = [g.new_edge_property("string") for _ in uniswapv2_info]

# UniswapV3 props
uniswapv3_info = ["tick", "liquidity", "sqrtPrice"]
uniswapv3_eprops = [g.new_edge_property("string") for _ in uniswapv3_info]

def get_eprops(pool_variant):
  if pool_variant == PoolVariant.UniswapV2:
    return uniswapv2_info, fixed_eprops + uniswapv2_eprops
  elif pool_variant == PoolVariant.UniswapV3:
    return uniswapv3_info, fixed_eprops + uniswapv3_eprops
  else:
    print("No correct eprops found for the given pool variant.")
    exit()

# ------------- Dex variant and pair retrieval logic
class PoolVariant(Enum):
  UniswapV2 = "UniswapV2"
  UniswapV3 = "UniswapV3"
class Dex:
  def __init__(self, dex_name, factory_address, pool_variant, graph_id, creation_block):
    self.dex_name = dex_name
    self.factory_address = factory_address
    self.pool_variant = PoolVariant(pool_variant)
    self.graph_id = graph_id
    self.creation_block = creation_block
  
  def get_pairs(self):
    if self.pool_variant == PoolVariant.UniswapV2:
      return self.get_pairs_uniswapv2_variant()
    elif self.pool_variant == PoolVariant.UniswapV3:
      return self.get_pairs_uniswapv3_variant()
    else:
      print("No pair finder function found for the given pool variant.")
      exit()

  def get_pairs_uniswapv2_variant(self):
    # Code for handling UniswapV2 variant
    pairs = []
    url = "https://api.thegraph.com/subgraphs/name/{}".format(self.graph_id)
    for i in range(PAGINATION):
      skip = 1000*i
      variables = {"skip": skip, "first": FIRST}
      # this extra info describes any other fields we may wish to ask for
      extra_info = ["reserve0", "reserve1"]
      query = f"""
        query pairs($skip: Int) {{
          pairs(first: $first, skip: $skip, orderBy: volumeUSD, orderDirection: desc, where: {{
            txCount_gt: 10
          }}) 
          {{
            {' '.join(extra_info)}
            token0 {{ id decimals }}
            token1 {{ id decimals }}
            id
          }}
        }}
        """
      pairs_resp = requests.post(url,json={'query': query, 'variables': variables}).json()['data']['pairs']
      pairs = pairs + pairs_resp

    # Add tags
    updated_pairs = [{**item, "variant": self.pool_variant.name, "dex": self.dex_name} for item in pairs]
    return updated_pairs

  def get_pairs_uniswapv3_variant(self):
    # Code for handling UniswapV3 variant
    pools = []
    url = "https://api.thegraph.com/subgraphs/name/{}".format(self.graph_id)
    for i in range(PAGINATION):
      skip = 1000*i
      variables = {"skip": skip, "first": FIRST}
      # this extra info describes any other fields we may wish to ask for
      extra_info = ["tick", "liquidity", "sqrtPrice"]
      query = f"""
        query pools($skip: Int) {{
          pools(first: $first, skip: $skip, orderBy: volumeUSD, orderDirection: desc, where: {{
            txCount_gt: 10
          }}) 
          {{
            {' '.join(extra_info)}
            token0 {{ id decimals }}
            token1 {{ id decimals }}
            id
          }}
        }}
        """
      pools_resp = requests.post(url,json={'query': query, 'variables': variables}).json()['data']['pools']
      pools = pools + pools_resp  
    
    # Add tags
    updated_pairs = [{**item, "variant": self.pool_variant.name, "dex": self.dex_name} for item in pools]
    return updated_pairs

# ------------- Data cleaning for thegraph responses
def get_uniswapv2_details(pair):
  # Token 0
  reserves0 = pair['reserve0']
  decimals0 = pair['token0']['decimals']
  precision0 = len(reserves0)
  getcontext().prec = precision0
  reserves0 = hex(int(Decimal(reserves0)*(10**int(decimals0))))
  
  # Token 1
  reserves1 = pair['reserve1']
  decimals1 = pair['token1']['decimals']
  precision1 = len(reserves1)
  getcontext().prec = precision1
  reserves1 = hex(int(Decimal(reserves1)*(10**int(decimals1))))
  
  token0 = Web3.to_checksum_address(pair['token0']['id'])
  token1 = Web3.to_checksum_address(pair['token1']['id'])
  
  details = {
    "reserves0": reserves0,
    "reserves1": reserves1
  }
  
  return token0, token1, details

def get_uniswapv3_details(pair):

  token0 = Web3.to_checksum_address(pair['token0']['id'])
  token1 = Web3.to_checksum_address(pair['token1']['id'])
  
  extra_info = ["tick", "liquidity", "sqrtPrice"]
  details = {prop: pair[prop] for prop in extra_info}

  return token0, token1, details

def extract_pair_details(pair):
  # Find the varient to know what to pull out
  varient = PoolVariant(pair['variant'])
  
  if varient == PoolVariant.UniswapV2:
    return get_uniswapv2_details(pair)
  elif varient == PoolVariant.UniswapV3:
    return get_uniswapv3_details(pair)
  else:
    print("No pair detail extractor function found for the given pool variant.")
    exit()

# ------------- Forming the pairsToTokens and the graph object
def make_graph(pairs):
  count = 0
  tokens = {}
  vertices = {}
  pairs_to_tokens = {}
  tokens_to_pairs = {}

  # Iterate through pairs
  for pair in pairs:
    # Check the pair address
    pair_address = Web3.to_checksum_address(pair['id'])
    # Check that the pair is not blacklisted
    if pair_address not in blacklist_pairs:
      
      # Get pair details
      token0, token1, details = extract_pair_details(pair)
            
      # Add token to nodes of graph if not seen
      if token0 not in tokens:
        tokens[token0] = count
        vertices[count] = token0
        g.add_vertex()
        count+=1
      if token1 not in tokens:
        tokens[token1] = count
        vertices[count] = token1
        g.add_vertex()
        count+=1

      # Add the pair as a node - edge - node connection
      v0 = g.vertex(tokens[token0])
      v1 = g.vertex(tokens[token1])
            
      info, eprops = get_eprops(PoolVariant(pair['variant']))

      g.add_edge_list([(v0, v1, token0, token1, pair_address, *[details[prop] for prop in info])], eprops=eprops)
      
      # Add it to our list(dict) to dump as json later
      pairs_to_tokens[pair_address] = {"pairInfo": {"token0": token0, 
                                                    "token1": token1, 
                                                    **details,
                                                    "dex": pair['dex'],
                                                    "variant": pair['variant']
                                                    }, 
                                      "cycles": []}
      
      tokens_to_pairs[pair['dex']+":"+token0+":"+token1] = pair_address
  return pairs_to_tokens, tokens_to_pairs

# ------------- Calculating paths
# TODO: put length of cycles into constants
def find_all_paths(pairs_to_tokens):
  paths = []
  for path in all_paths(g, 1, 1, cutoff=4):
    if len(path) <= 4:
      if len(path) == 3:
        edges = g.edge(path[0], path[1], all_edges=True)
        if len(edges) == 2:
          paths.append(path)
      if len(path) == 4:
        paths.append(path)
  
  for path in paths:
    if len(path) == 3:
      edges = g.edge(path[0], path[1], all_edges=True)
      if len(edges) == 2:
        pairAddress0 = pair_prop[edges[0]]
        pairAddress1 = pair_prop[edges[1]]
        
        cycle = [pairAddress0, pairAddress1]
        for pairAddress in cycle:
          if cycle not in pairs_to_tokens[pairAddress]['cycles']:
              pairs_to_tokens[pairAddress]['cycles'].append(cycle)
        cycle = [pairAddress1, pairAddress0]
        for pairAddress in cycle:
          if cycle not in pairs_to_tokens[pairAddress]['cycles']:
              pairs_to_tokens[pairAddress]['cycles'].append(cycle)
    else:
      cycle = []
      for i in range(len(path)-1):
        edge = g.edge(path[i], path[i+1])
        pairAddress = pair_prop[edge]
        cycle.append(pairAddress)
      for pairAddress in cycle:
        if cycle not in pairs_to_tokens[pairAddress]['cycles']:
          pairs_to_tokens[pairAddress]['cycles'].append(cycle)

# ------------- Debug graph function
def print_graph_details():
  # Print the number of vertices and edges in the graph
  print("Number of vertices:", g.num_vertices())
  print("Number of edges:", g.num_edges())

  # Print the degrees of all vertices in the graph - SPAMMY
  # print("Degrees of vertices:")
  # for v in g.vertices():
  #     print("Vertex", int(v), "degree:", v.out_degree())

  # Print the graph properties (such as directedness, weightedness, etc.)
  print("Graph properties:")
  print("Is directed:", g.is_directed())

  # Print other graph statistics using graph_tool's built-in functions
  print("Average degree:", vertex_average(g, "out"))
  print("Clustering coefficient:", global_clustering(g))
  print("Diameter (approximation):", pseudo_diameter(g)[0])

def main():
    
  # Create a list to store Dex objects
  dexes = []
  
  # Add dexes we want to play with
  
  # Add UniswapV2 pairs
  dexes.append(Dex(
    "UniswapV2",
    "0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f", 
    "UniswapV2",
    "uniswap/uniswap-v2",
    10000835))

  # Add Sushiswap pairs
  dexes.append(Dex(
    "Sushiswap",
    "0xC0AEe478e3658e2610c5F7A4A2E1777cE9e4f2Ac", 
    "UniswapV2",
    "zippoxer/sushiswap-subgraph-fork",
    10794229))

  # Add UniswapV3 pools
  dexes.append(Dex(
    "UniswapV3",
    "0x1F98431c8aD98523631AE4a59f267346ea31F984", 
    "UniswapV3", 
    "uniswap/uniswap-v3",
    12369621))

  # Fill the Dex objects with pairs / pools
  pairs = [result for dex in dexes for result in dex.get_pairs()]
  
  # Create tracking dicts and create the graph
  pairs_to_tokens, tokens_to_pairs = make_graph(pairs)
  
  # Use the graph to find paths and add them to the pairs_to_tokens cycles
  find_all_paths(pairs_to_tokens)
  
  # Print graph details
  # print_graph_details()
  
  # Dump the JSON so it is readable in C2 bot
  # Get the directory where the script is located
  script_dir = os.path.dirname(os.path.abspath(__file__))
  json_file_path = lambda filename: os.path.join(script_dir, filename)
  
  with open(json_file_path('pairsToTokens.json'), 'w') as f:
    json.dump(pairs_to_tokens, f) #, indent=4)
  with open(json_file_path('tokensToPairs.json'), 'w') as f:
    json.dump(tokens_to_pairs, f) #, indent=4)
  

# Call the main function if the script is executed directly
if __name__ == "__main__":
    main()

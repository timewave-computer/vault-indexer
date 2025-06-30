import { ethers as Ethers, BlockTag, JsonRpcApiProviderOptions } from "ethers";

const rpcUrl = process.env.API_ETH_RPC_URL;
if (!rpcUrl) {
  throw new Error('API_ETH_RPC_URL is not set');
}

const getEthersProvider = () => {
  // Configure provider options to prevent retries
  const options: JsonRpcApiProviderOptions = {
    batchMaxCount: 1,        // Disable batching which can cause retry loops
    polling: false,        // Disable polling if you don't need events
    staticNetwork: true,   // Use if network is known and won't change
  };

  return new Ethers.JsonRpcProvider(rpcUrl, {
    name: 'Ethereum',
    chainId: 1,
  }, options);
}

const ethersProvider = getEthersProvider()

export const getMostRecentBlockNumber = async (
    blockTag?: BlockTag
): Promise<number> => {

  const block = await ethersProvider.getBlock(blockTag ?? "latest");
  if (!block) {
    throw new Error('Could not fetch finalized block');
  }
  return block.number;
};




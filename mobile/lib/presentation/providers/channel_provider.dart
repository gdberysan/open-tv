import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../domain/models/channel.dart';
import '../../domain/models/channel_filter.dart';
import '../../data/repositories/channel_repository.dart';

final channelRepositoryProvider = Provider<IChannelRepository>((ref) {
  return ChannelRepository();
});

final channelFilterProvider = StateProvider<ChannelFilter>(
  (_) => const ChannelFilter(),
);

final channelsProvider = FutureProvider<List<Channel>>((ref) async {
  final filter = ref.watch(channelFilterProvider);
  final repo = ref.watch(channelRepositoryProvider);
  return repo.getChannels(filter: filter);
});

final streamUrlProvider =
    FutureProvider.family<String, String>((ref, channelId) async {
  final repo = ref.watch(channelRepositoryProvider);
  return repo.getStreamUrl(channelId);
});

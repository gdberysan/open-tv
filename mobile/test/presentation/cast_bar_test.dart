import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:korven_open_tv/domain/models/cast_session.dart';
import 'package:korven_open_tv/presentation/providers/cast_provider.dart';
import 'package:korven_open_tv/presentation/widgets/cast_bar.dart';

class _CastFijo extends CastNotifier {
  _CastFijo(this._inicial);
  final CastSession _inicial;

  @override
  CastSession build() => _inicial;
}

Future<void> _montar(WidgetTester tester, CastSession sesion) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [castProvider.overrideWith(() => _CastFijo(sesion))],
      child: const MaterialApp(home: Scaffold(body: CastBar())),
    ),
  );
}

void main() {
  testWidgets('en idle no se dibuja', (tester) async {
    await _montar(tester, const CastSession());
    expect(find.textContaining('AirPlay'), findsNothing);
    expect(find.byIcon(Icons.airplay), findsNothing);
  });

  testWidgets('armed muestra el dispositivo', (tester) async {
    await _montar(
        tester,
        const CastSession(
            state: CastState.armed, deviceName: 'Salón Apple TV'));
    expect(find.textContaining('Salón Apple TV'), findsOneWidget);
  });

  testWidgets('casting muestra dispositivo y canal', (tester) async {
    await _montar(
      tester,
      const CastSession(
        state: CastState.casting,
        deviceName: 'Salón Apple TV',
        channelName: 'BBC News',
      ),
    );
    expect(find.textContaining('Salón Apple TV'), findsOneWidget);
    expect(find.textContaining('BBC News'), findsOneWidget);
  });

  testWidgets('sin nombre de dispositivo cae a AirPlay', (tester) async {
    await _montar(tester, const CastSession(state: CastState.armed));
    expect(find.textContaining('AirPlay'), findsOneWidget);
  });
}
